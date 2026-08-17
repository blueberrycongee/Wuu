# AI-味 A/B 实验：一句风格提示的效果

模型：deepseek-v4-pro（wuu CLI 驱动，provider `deepseek`，用户默认配置）
工作区：wuu 仓库根目录
日期：2026-08-17

## 提示词（唯一差异 = 一句话）

- A（无风格提示）：
  `写一篇关于 wuu 裁剪投影（tool result projection）的技术报告，减少 AI 味。直接输出报告全文，不要创建文件。`
- B（+Anthropic 风格提示）：
  `写一篇关于 wuu 裁剪投影（tool result projection）的技术报告，参考 Anthropic 的工程博客风格去写，减少 AI 味。直接输出报告全文，不要创建文件。`

## 运行命令（两侧对称）

```
./wuu exec --provider deepseek --model deepseek-v4-pro \
  --permission-mode read_only --max-turns 20 --timeout 25m --ephemeral \
  --json --output-last-message <out>.md "<prompt>"
```

## 产物

- `a_report.md`：A 组最终报告（96 行 / 8.4KB）
- `b_report.md`：B 组最终报告（108 行 / 9.7KB）

## 记录

- B 组首次以 `--max-turns 12` 运行超限失败（agent 仍在读代码/写作中），
  随后 A、B 均以 `--max-turns 20` 重跑，保证预算对称。
- 首次 A 组（12 轮）产出的报告比 A-20 更叙事化——单次运行有采样方差，
  n=1 对比只能作定性参考。
