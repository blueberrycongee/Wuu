报告基于代码库实读写成——[result_projection.go](internal/tools/result_projection.go)、[result_projectors.go](internal/tools/result_projectors.go)、[result_budget.go](internal/tools/result_budget.go)、[tool_continuation.go](internal/tools/tool_continuation.go) 和 provider 侧的投影逻辑。全文如下。

---

# 工具结果投影：执行时裁剪一次，而不是每次请求裁剪一遍

Wuu 是一个本地编码 agent：Go 内核跑工具循环，Electron 桌面端做交互。模型每个回合看到对话历史加上一批工具结果，而工具结果是上下文里最膨胀的部分——一个 grep 可以返回几万条匹配，一次 read_file 能读出半个依赖目录，bash 的 stdout 是完整构建日志。字节进上下文要花三样东西：token 费、推理延迟、注意力。所以每个 agent 产品都必须回答同一个问题：工具结果太大时，裁哪里。

我们以前的做法是请求时裁剪：每次组装模型请求，把历史里旧的工具输出按预算重新砍一遍。它有两个毛病。第一，同一个工具结果，第 N 次请求和第 N+1 次请求看到的字节不一样，历史前缀被不断改写，provider 端的 prompt cache 每轮全部失效。第二，裁剪逻辑在请求组装层，离工具语义太远，只能按字符和行机械切割，结构说破坏就破坏。

现在换成：执行时投影一次，存储前完成，之后永不改写。

## 核心机制

一句话：工具结果在执行完成的那一刻，被投影成一份有界的、懂工具结构的、带 artifact 引用的形式；投影发生在结果进历史之前，所以模型在第一次请求和同一 cache epoch 内每一次后续请求看到的字节完全相同。完整结果不丢——落盘成会话 artifact，投影里带着把丢掉的部分读回来的路径。历史是 append-only 的，第 N 条消息不会因为第 N+1 条出现而改变。

这跟"压缩"（compaction）是两件事。投影在单个结果粒度上运行，压缩在整段历史粒度上运行；投影保证压缩之前的那段窗口是稳定可缓存的，压缩仍然会改写前缀，但频率从"每请求一次"降到"每若干轮一次"。

## 两条边界

Wuu 有两个不同精度的机制，别混：

**工具专用投影（projection）。** 只有六个内置工具在精确匹配的 allowlist 上：`read_file`、`grep`、`glob`、`list_files`、`bash`、`thread_get`。预算是 2048 个估算 token。投影器懂自己工具的 JSON envelope，只丢整条记录或整行，从不切序列化好的 JSON 字节。

**通用结算（generic settlement）。** 所有其它工具的兜底：超过 50K 字符（约 12K token）或 2000 行的结果整段落盘，模型收到文件路径、4KB 预览和结构化索引。

allowlist 用精确名字匹配是有原因的。MCP 工具名永远带 `mcp_<server>_` 前缀，registry 又先于 MCP 解析裸名，所以裸名不可能落到第三方工具上；改成前缀匹配会立刻出错——`mcp_x_bash` 会匹配上 `bash`。同理，`apply_patch`、`edit_file` 这类变更工具和 `load_skill` 这类协调工具永远不会被投影：丢了它们的记录就丢了任务状态本身。

## 投影器如何工作

每个投影器很小，但都遵守三条规则。

**只丢可数单位。** glob、list_files、grep 的主体是一个记录数组，投影器用二分搜索（`largestFitting`）找出预算内能装下的最大前缀，然后同步改写 `returned_count`、`has_more` 和 `page`，让信封自洽。grep 内容模式额外维护 `returned_match_count` / `omitted_match_count`，模型看到的计数必须和实际剩余内容一致。read_file 保留头部和尾部整行，中间换成省略标记，绝不把一行切两半。

**确定性。** 解析用 `json.Number` 保数字精度，序列化靠 map key 排序，同一输入必然产生同一字节。投影器有版本号（现在是 `"2"`），任何改变投影字节的修改都必须 bump，否则 telemetry 无法把一次投影归因到具体版本。

**失败打开（fail-open）。** envelope 解析不了、字段缺失、artifact 写不进磁盘——任何一条路径失败，返回的就是原始结果，一个字节不少。原则很简单：模型多读几百字节不会死，但拿到一个指向不存在文件的引用会。

bash 的投影器是这三条规则的集中体现。它先丢冗余——envelope 里已经重复出现的输出；然后丢 stdout 尾部；最后才动 stderr。代码注释的原话是："verification/metadata 是拒绝丢弃的证据。"

## 顺序：先保证可恢复，再丢证据

顺序是这个设计里最硬的部分：投影器跑之前，先落盘 artifact。artifact 写不进去，整个投影放弃，返回原样。不允许出现"投影成功但引用死了"的中间态。

bash 有个优化：它的 envelope 里本来就有 `full_log_ref`，直接复用，不落第二份拷贝。

恢复路径有三条，对应三种工具形状：

- **read_file** 给 `continuation.next`，里面是 base64 编码的字节区间——`path`、`byte_offset`、`limit`、`expected_sha256`。模型可以用 read_file 直接读回被省略的中间段，SHA 校验保证读回来的是投影时看到的同一份内容。
- **glob / list_files / grep** 给 `page.next`：`offset` 加 `expected_revision`。翻页偏移绑定到产生上一页的结果集 revision；文件在两次调用之间增删导致 revision 变化时，continuation 被拒绝，错误信息让模型 "restart at offset 0"。宁可重来，不许静默重复或跳记录。
- **bash** 给 `ranked_artifact_ranges`：stderr 和 stdout 各一段字节区间，stderr 排在前面。

一份真实的 read_file 投影长这样：

```json
{
  "action": "read_file",
  "path": "internal/tools/result_projection.go",
  "content": "<头部整行>\n... omitted 172 lines; use continuation.next ...\n<尾部整行>\n",
  "truncated": true,
  "projection": {
    "projected": true,
    "budget_tokens": 2048,
    "artifact_ref": "/…/tool-results/call_00_….txt",
    "omitted_lines": 172,
    "shown_lines": 136
  },
  "continuation": {
    "has_more": true,
    "next": { "continuation": "eyJwYXRoIjoi..." }
  }
}
```

模型在投影里看到的每样东西都是可行动的：省略了多少、为什么省略、从哪读回来。

## thread_get 是例外

六个工具里 thread_get 特殊：它永远走投影器，即使原始结果没超预算。原因是它的投影不是 oversized 优化，而是语义读取契约——thread_get 返回的语义页（context anchor 加最近若干轮完整 turn，带向更旧方向翻页的游标）就是它的产品形态。上下文 anchor 的裁剪上限被钉在共享预算的三分之一，防止一个巨大的 compact summary 吃掉全部字节，同时避免引入第二个字节限制。

## 富内容和结构化数据

投影只作用于 model-visible 文本。图片、音频、文件附件在 provider 侧投影成一条隐藏的 observation message，不在文本里塞 base64。结构化内容生成一个有界语义索引：`shape`、`key_count`、前 64 个 key、`item_count`、`content_sha256`、`original_characters`、`continuation`——完整的结构化 payload 留在 durable ToolResult 上，不上 wire。checkpoint 压缩时用同一套索引规则，保证结构不丢。

## 影子模式和观测

投影有三种模式：`off`、`shadow`、`active`，默认 `active`，环境变量 `WUU_TOOL_RESULT_PROJECTION` 可覆盖。shadow 模式下投影照算照记，但模型看到的仍是通用结算结果，这是给 A/B 测量留的。

诊断记录（`ProjectionDiagnostics`）只含非内容事实：工具名、call_id、预算、是否 eligible、是否 applied、reason 标签、原始/投影后的字节数和 token 估算、两个 SHA-256、artifact 状态、省略计数。不含一行工具输出文本。reason 是低基数稳定标签——`not_eligible`、`under_budget`、`no_projector`、`projected`、`fail_open`——跨运行可以直接对比。

## 数字和开放问题

2048 这个预算怎么来的？本地语料上的实测：2K 预算触及约 13% 的 eligible 结果，砍掉约 24% 的 token。代码注释的原话是："它是实验起点，不是被证明的最优值；跨工具共享预算是为了减少变量。"

token 估算用的是共享的确定性估算器，不是 provider 的 tokenizer。它和真实计费 token 有偏差，但确定性本身比精确更重要——投影路径和结算路径必须用同一把尺子，否则影子测量没法做。

我们刻意不做语义摘要。投影只做结构层面的缩减，不重写语义：不调模型、不引入非确定性、不把摘要质量变成投影器版本的一部分。代价是长尾由 checkpoint 压缩接住。0.4.0 的 changelog 用一句话概括了这个分工："请求时的历史工具输出裁剪被移除。上下文增长现在由稳定投影加 checkpoint 压缩处理，普通工具循环请求保持 append-only 的可缓存前缀。"

## 不变式清单

整个机制归结为五条不变式：

1. 投影发生在执行时、存储前，只发生一次。
2. 历史只增不改：同一 cache epoch 内，投影字节稳定。
3. 丢证据之前必须有可恢复引用；artifact 写不进就整体放弃。
4. 失败打开：任何路径失败，返回原始结果。
5. 投影只碰 model-visible 文本；媒体、结构化元数据和 durable 状态原样保留。

每一条在测试里都有专门盯着的用例。剩下的开放问题——每工具独立预算、按 provider 校准 token 估算、投影对任务完成质量的实际影响——是下一阶段的事。