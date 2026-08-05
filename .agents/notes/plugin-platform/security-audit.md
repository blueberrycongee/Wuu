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
3. **控制 #4 缺测试**：bundled provenance 不可被 user/project 同 id 遮蔽——有 project>user 优先级测试，但无「user/project 包试图 shadow bundled id」的对抗测试。
4. **控制 #8 缺测试**：插件身份由宿主连接绑定、调用方自报 id 无权威——未见 spoofing fixture。
5. **控制 #10 缺测试**：disable/revoke 后 stale registration 清理（命令/MCP/hook 残留）未见专项 fixture（reconcilePluginHost 存在但无对抗用例）。

## 建议

- 缺口 1+2 是架构层（hook 权限目录 + 载荷裁剪），建议纳入 Andy 总架构下一阶段 manifest 设计，权限目录候选：`session.read`（chat.message/chat.request 正文）、`session.write`（chat.request 改写）、`tools.intercept`（tool.*）、`shell.env`。未声明即剥离注册或降级载荷。
- 缺口 3+4+5 是纯测试补网，不碰实现，我申领（等房间确认后动手；若测出实 bug 另行上报）。
