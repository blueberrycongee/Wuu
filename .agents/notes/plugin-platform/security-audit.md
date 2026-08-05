# 安全轨独立审计 — package approval platform (575b53b3 / 8a53f866)

审计人：秦始皇（安全/测试分轨）。方法：对照 threat-model.md 15 条控制逐条核实实现与测试覆盖，并独立运行测试。

## 验证结果

`go test ./internal/plugin/... ./internal/pluginhost/... ./internal/extensions/... ./internal/runtime/ -count=1` 全绿（本机，2026-08-05）。

## 已落地且覆盖良好的控制

| 控制 | 实现 | 测试 |
|---|---|---|
| #6 最小环境 | `buildEnv` baseline 白名单，不继承 os.Environ（process.go:281） | TestProcessEnvDoesNotInheritSecrets |
| #1 审批前惰性 | `activatedPlugins` 门：community 默认不激活、official provenance 激活、指纹变化即停、权限不足即停 | plugin_activation_test.go |
| #3 指纹失效 | grant 精确指纹更新/过期拒绝/并发 worker 拒绝 | extension_package_test.go |
| #5 未知字段 fail closed | 未知声明 hook 拒绝 | TestProcessClientRejectsUnknownDeclaredHook |
| #9 死线/尺寸限制 | 超时 + oversize response + bounded stderr | process_test.go |
| #15 符号链接逃逸 | manifest 资产 symlink 解析后限包根 | path_security_test.go |
| #2 内置 provenance | bundled 隐藏/平台过滤/显式启用 | plugin_test.go |

## 缺口（按威胁模型逐条核对）

1. **控制 #7 只到包级，未到 hook 级**。当前 `permissionSetContains(grant.Permissions, item.EffectivePermissions)` 是整包激活门；没有「hook → 所需权限」的独立映射（threat-model「Permission behavior」节要求 runtime host independently maps each surface to required permissions）。后果：一个获批插件自动获得全部 8 个 hook 的完整能力，包括 `chat.request` 全量会话读写与 `shell.env` 注入。
2. **观测 hook 载荷未最小化**。chat.* 观测路径仍给完整消息正文，没有 metadata-only 降级（threat-model：observation hooks receive immutable or minimized payloads）。

## 已补对抗测试并验证通过（本轮新增）

- 控制 #4：`internal/plugin/shadowing_test.go`——project/user 包以同 id 声明恶意 runtime 试图遮蔽 bundled cua-mac，两条用例均证明 provenance 获胜（PASS）。
- 控制 #10：`TestReconcilePluginHostClosesRevokedPluginClient`——grant 撤销后 client 被 Close 且从 host 注销，存留插件不受影响（PASS）。
- 控制 #3：`TestReconcilePluginHostRestartsClientWhenFingerprintChanges`——指纹变化不复用旧 client，关闭并换新（PASS）。
- 控制 #8：核实为**结构性满足，无需 fixture**——线上协议 `rpcResponse` 没有 id 字段，插件身份由宿主持有的 `ProcessConfig.ID` 与独占管道绑定，调用方自报 id 在协议上不存在入口；initialize 的未知/重复 hook 已有拒绝测试。

## 建议

- 缺口 1+2 是架构层（hook 权限目录 + 载荷裁剪），建议纳入 Andy 总架构下一阶段 manifest 设计，权限目录候选：`session.read`（chat.message/chat.request 正文）、`session.write`（chat.request 改写）、`tools.intercept`（tool.*）、`shell.env`。未声明即剥离注册或降级载荷。
- 缺口 3+4+5 已闭环：#4/#10/#3 补网通过，#8 结构性满足。安全轨对抗 fixture 验证策略的存量部分完成，后续随 hook 级权限落地补「未声明权限注册敏感 hook 被拒/载荷被裁剪」用例。
