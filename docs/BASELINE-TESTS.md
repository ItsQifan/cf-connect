# 基线测试状态（trim 改造前，commit 3a6534d + web/dist 构建产物）

采集环境：Windows 11 / go1.25.14 windows/amd64 / Node v22.22.0 / pnpm 10 (corepack)
采集命令：`go test ./...`

## 结论

基线本身在 Windows 上**并非全绿**。下列失败均为**上游既有问题**（与本次裁剪无关），
只做记录，不作为本次改造的引入项。改造后要求：这些失败不应**新增**。

## 基线失败清单

| 包 | 失败用例 | 根因（实测） | 类别 |
|---|---|---|---|
| `agent/acp` | TestAgyPermissionBridgePreservesHooksAndRelaysDecisions | 平台相关 | 上游 |
| `agent/antigravity` | TestSendDoesNotHoldStdinOpen | 平台相关 | 上游 |
| `agent/antigravityhook` | TestRunHookCommand / TestTryHook | 依赖 claude CLI | 上游 |
| `agent/claudecode` | 8 例（TestNew_*、TestSkillDirs_*、TestWriteTempAppendPromptFile_*） | 依赖 claude CLI / POSIX 权限 | 上游 |
| `agent/codex` | 4 例（TestSend_*、TestGetModelAndReasoningEffort_*） | 依赖 codex CLI / PTY | 上游 |
| `agent/cursor` | TestListCursorSessions_ConfigCursorPath / TestSkillDirs_IncludesCursorAndClaudePaths | HOME 环境 | 上游 |
| `agent/iflow` | TestIFlowSessionPendingToolTimeoutClearsBusyState | 时序 | 上游 |
| `agent/kimi` | 3 例 | HOME / 路径 | 上游 |
| `agent/opencode` | 21 例（TestAvailableModels_* 等） | 动态模型发现依赖真实 CLI 行为 | 上游 |
| `agent/pi` | 9 例 | 平台相关 | 上游 |
| `cmd/cc-connect` | **构建失败**：`doctor_runas_test.go` 无 `//go:build !windows`，而 `doctor_runas.go` 有 | 上游缺 build tag | 上游 |
| `config` | TestLoad_DefaultsDataDir / TestLoad_ResolvesEnvPlaceholders | 测试未隔离 HOME（读到真实 `C:\Users\qifan`） | 上游 |
| `core` | TestCompactReplyFooterPath_HomeRelativeDeepPathStaysFull / TestCmdShell_MultiWorkspaceIgnoresMissingSharedBinding / TestAppendImageRefs / TestAppendFileRefs_* (3) | Windows 路径分隔符断言 | 上游 |
| `daemon` | TestSchtasksInstall_TightensExistingScriptFrom0644 | Windows 无 POSIX 0644 语义 | 上游 |

## 与本次改造的关系

- 阶段 1/2 会**删除** `agent/*`（除 opencode）与 `platform/*`（除 dingtalk）→ 上表多数条目随之消失。
- 保留包中仍存在的上游失败：`agent/opencode`(21)、`cmd/cf-connect`(1，构建修好后才暴露)、
  `config`(2)、`core`(4 稳定 + 2 不稳定)、`daemon`(1)。
- 其中 `cmd/cc-connect` 的构建失败**阻塞测试**，本次随手用一行 build tag 修掉（低风险、必要）；
  修好后该包暴露出 1 个上游既有失败 `TestParseDaemonInstallArgs_WorkDirOverridesConfig`
  （`"/tmp/override"` vs `"\\tmp\\override"`，Windows 路径分隔符断言），未修。
- 其余保留包的上游失败**不在本次范围**（计划未列），保持原状并在 §验收中如实说明。

## 不稳定（flaky）用例

改造后首轮全量 `go test ./...` 曾出现两条**基线里没有**的失败，单独跑即通过、
重跑全量也不再出现，判定为时序/端口竞争导致的不稳定，非本次改造引入：

| 用例 | 首轮现象 | 复核 |
|---|---|---|
| `core.TestCUJ_A5_FileReachesAgent` | 全量并发下失败 | 单独跑 PASS；第二次全量 PASS |
| `core.TestTimerScheduler_FiresOnTime` | 耗时 91s 后失败（等定时器触发） | 单独跑 PASS；第二次全量 PASS |

同时 `core.TestCmdShell_MultiWorkspaceIgnoresMissingSharedBinding` 在基线里失败、
本次两次全量中一次 PASS 一次 FAIL —— 同属不稳定，已在上表按"上游 + 不稳定"记录。

**结论**：改造后的稳定失败集合 = 基线稳定失败集合（去掉被删除的包），无新增。

