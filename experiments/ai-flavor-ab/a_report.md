# Wuu 工具结果裁剪投影（tool result projection）技术报告

## 1. 术语范围

报告里"投影"指两件不同但接续的事，代码里也是分开的两层：

- **执行时结算（settlement）**：工具刚返回、结果落库之前，把超限的内置工具结果一次性改写成有界、可恢复的形式。代码在 `internal/tools/result_projection.go`、`result_projectors.go`、`result_budget.go`，调用点在 `internal/tools/tool_telemetry.go` 的 `executeToolCall` 收尾段。
- **请求时投影（provider projection）**：每次组装发给模型的请求时，把持久化的富结果转换成 provider 协议格式。代码在 `internal/providers/tool_result_projection.go`，入口是 `message_request.go` 里的 `ApplyToolResultProjections`。

## 2. 要解决的问题

两条，其中第二条决定了两层的先后关系。

**上下文膨胀。** `bash`/`grep` 的失控输出、`glob`/`list_files` 的大目录、`read_file` 的大文件、`thread_get` 的完整会话快照，单条结果就能到几万 token。不截断，一次调用吃掉大半个窗口；硬截断，模型拿到的是残缺信息，且没有恢复原样的路径。

**前缀缓存失效。** 早期做法是在请求时修剪历史里的工具输出：每次请求按当天预算重新推导一遍历史。同一段历史在不同请求里字节不同，provider 侧的 prompt 前缀缓存对不上，每个请求全额重算。0.4.0 起删掉了这条请求时修剪路径（CHANGELOG 有记录）。

两条合起来的结论：裁剪必须发生在执行时、落库之前，让存储下来的形式就是模型最终看到的形式。

## 3. 一条结果的路径

工具返回 → `executeToolCall` 结算（工具专用投影或通用结算，二选一）→ 落库 → 请求组装时 `ApplyToolResultProjections` → provider 电线格式。

结算层和请求层各干各的：结算层决定"存什么"，请求层决定"怎么发"。结算层只跑一次，请求层每次请求都跑。

## 4. 结算层

### 4.1 资格

- 白名单精确匹配：`read_file`、`grep`、`glob`、`list_files`、`bash`、`thread_get`。
- 不能用前缀匹配。MCP 工具名统一带 `mcp_<server>_` 前缀，前缀匹配会让 `mcp_x_bash` 撞上 `bash`。变更类（`apply_patch`、`edit_file`）和协调类（`load_skill`）也不在名单里。
- 只处理 text-only 结果；带媒体或富结构的结果走通用结算。

### 4.2 预算

每结果 2,048 估计 token，和 shell/grep 工具自身的输出上限同量级。本地语料上，这个预算碰到了约 13% 的合格结果，去掉了这些结果约 24% 的 token。代码注释写明这是实验起点而非验证过的最优值，各工具共用一个预算是为了减少变量。

### 4.3 工具专用投影器

原理统一：解析工具自己的 JSON envelope（`json.Number` 保数字精度），**删整条记录或整行，从不切序列化后的 JSON**——切 JSON 会破坏结构。保留 envelope 里的标量证据（counts、revision、flags），追加一个 `projection` 对象指向可恢复工件。预算拟合用 `largestFitting` 二分，依赖 size(keep) 单调不减。

各工具的差别：

- `bash`：优先保 stderr，先裁 stdout；裁剪保留最近的行；冗余输出先删；verification/metadata 是"拒绝丢弃的证据"，宁可最终仍略超预算。续读走 `full_log_sections` 的字节区间，按流给 ranked ranges。
- `thread_get`：特殊在它是 **always project**——投影不是超限优化，而是它的读取契约本身（分页 + `page.next`）。所以即使原始结果没超预算也走投影器。上下文锚点封顶在预算的 1/3；长文本用头尾保留加 `… N runes omitted …` 中缝标记；请求更老的分页要求 `before_seq`/`snapshot_seq`/`snapshot_token` 齐全，不齐就失败开放。
- 其余四个（`read_file`/`grep`/`glob`/`list_files`）按各自 envelope 的字段语义删行、删记录。

确定性来自三处：map key 排序序列化、`SetEscapeHTML(false)`、`json.Number`。相同输入产相同字节。`projectorVersion` 当前为 "2"，任何改变投影字节的改动必须 bump，telemetry 靠它归因到具体投影器版本。

### 4.4 可恢复性先于裁剪

`ensureProjectionArtifact` 在任何证据被丢之前先保证完整结果可恢复：

- `bash` 复用 envelope 里自带的 `full_log_ref`，不写第二份。
- 其他工具把原始文本落盘到会话目录 `tool-results/<callID>.txt`。
- 会话目录为空或写盘失败，就失败开放（fail open），原样保留完整结果。不允许出现指向不存在文件的死引用。

只有"投影成功"和"工件可恢复"两个条件同时成立，结果才被替换。

### 4.5 通用结算

不适用专用投影的结果全部过 `finalizeGenericToolResult`：

- 阈值 50K 字符（约 12K token）或 2,000 行。
- 超限后完整文本落盘，模型可见文本换成有界引用（路径 + 预览）。
- 富媒体和结构化元数据不动；Text/Resource/ResourceLink 三类部件在第一个文本位置统一替换。resource body 在 Wuu 里属于 provider 文本，必须一起归档，不能留成无界旁路。
- 结构化结果附语义索引：`archived_structured_tool_result` + `artifact_ref` + sha256 + shape/keys（key 最多 64）或 item_count，外加 `continuation.next` 字节区间续读。

### 4.6 模式与观测

`off` / `shadow` / `active`，默认 `active`；环境变量 `WUU_TOOL_RESULT_PROJECTION` 可覆盖。`shadow` 只计算投影和诊断并记录，模型仍看通用结算的结果，用于 A/B 测量。

诊断（`ProjectionDiagnostics`）只含尺寸、计数、哈希和稳定标签，不含工具输出文本。`reason` 五个值：`not_eligible` / `under_budget` / `no_projector` / `projected` / `fail_open`，低基数，跨运行可比。未超预算的合格结果走 identity 路径，投影哈希等于原始哈希，telemetry 能区分"投影了"和"原样通过"。

## 5. 请求时投影

`ApplyToolResultProjections` 把持久的 `toolresult.Result` 转成 provider 电线格式：

- 文本部件拼接进 tool 消息的 content；图片抽到 `ObservationImages`、音频/文件抽到 `ObservationFiles`，作为独立的隐藏 user 观察消息（"Tool result observations."）发出，而不是塞进 tool 消息。
- 不支持的部件类型降级为带 URI 的说明文本，不静默丢弃。
- 结构化内容：有内容部件的工具只带语义索引（与结算层同款），防止完整 payload 在电线上复制一份；纯结构化工具（没有自己的内容投影）带完整 canonical JSON。
- 空结果补占位文本（"Tool completed without a textual result." 等）；历史里没有富数据的旧消息也补，因为 provider 协议要求 tool 输出非空。

## 6. 设计不变量

1. **投影即存储。** 投影在落库前替换结果，历史 Content 和富 ToolResult 用同一个值，下游没有第二来源。模型在第一次请求和同一前缀缓存 epoch 内每次后续请求看到相同字节，历史前缀保持可追加缓存。
2. **失败开放。** 不合格、未超预算、投影器拒绝、工件写不进，全部原样返回。不存在半裁状态。
3. **历史不追溯改写。** 投影只作用于新结果。检查点压缩（compaction）是独立机制，两者都保留语义索引，混合结构化结果不会在压缩或请求时被完整复制进上下文。
4. **富结果跨持久化存活。** 结构化内容与附件元数据在重启、压缩后不丢（0.4.0 修过这个 bug）。

## 7. 已知边界与取舍

- 2,048 token 预算没有质量验证，`shadow` 模式就是留给这项测量的。
- 裁剪只作用文本。媒体不进 token 预算裁剪：结算层对媒体原样保留，通用结算只界文本。媒体的成本控制靠压缩和引用，不在这套机制里。
- MCP 结果、变更类结果不投影：MCP 走通用结算；`apply_patch` 的输出本身就是变更证据，裁了破坏语义。
- 续读依赖字节区间（`encodeReadFileByteContinuation`）和 thread 分页的 snapshot 对。epoch 推进后旧快照失效、续读失败是预期行为，不是缺陷。
- 预算、阈值、白名单都是单点常量；改投影字节必须同步 bump `projectorVersion`，否则 telemetry 归因会串。