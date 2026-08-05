# 在途改动预评审（安全轨，2026-08-06）

对象：Andy 未提交的 19 文件工作区 diff（+609/−50）。只评审，未改其文件。

## Go 侧结论：方向全部正确，无安全异议

1. **包哈希加固**（package_contract.go）：条目总数上限 10k、总体积上限 128MiB、逐条目 symlink 解析后限包根、跨路径去重。补上威胁模型「失控/供应链」两类的资源耗尽角度；附 TestPackageContractRejectsEntryTreesOverByteLimit。
2. **发现失败显性化**（plugin.go：DiscoveryError + discoveryFailurePlugin）：manifest 加载错误不再被静默吞掉，变成带诊断的 inventory 记录。注意这部分此前也一直只在工作区——575b53b3 部分提交的又一缺失件，scanPluginRoots 在纯 HEAD 上是静默跳过坏包的。
3. **grants.Clone()**：深拷贝 Permissions 切片，防共享底层数组，正确。
4. **settings_layer protectUserSettings**：方向符合控制 #13，但当前版本误伤 local 层（见 integration-verification.md 的失败记录），等其修正。

## 与我已提交面的关系

无冲突。我 7235ae8b 补提交的 host.Clients/Replace 与其工作区版本一致；63adb873 的 variadic ApplyExtensionPolicy 与其在途调用方（二参版）兼容。

## 流程观察（第二次提）

在途 diff 已积累 19 文件且与 HEAD 多处互为依赖，继续积大会放大 rebase 与评审成本。建议尽快按「调用方+被调方」原子切片提交，每片过 pure-HEAD build。
