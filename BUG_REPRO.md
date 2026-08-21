# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

两个客户端几乎同时用同一个幂等键创建货运单时，后到的请求没有等待首个请求落库，直接拿到了 102 和空响应；首个请求最终成功后再重试才会得到 201。更麻烦的是首个请求失败时，并发请求也已经把 102 当成重放结果。请先不要修改代码，定位幂等执行协调失效的根因，说明完成、失败和等待者之间的状态时序与污染范围。

## 含 Bug 版本

- 仓库：VanceMichael/go-label-10
- 仓库地址：https://github.com/VanceMichael/go-label-10.git
- parent SHA：21cdd423480470857e3e074638d412c4d8b745f5

## 复现步骤

```bash
git clone -- https://github.com/VanceMichael/go-label-10.git bug-repro
cd bug-repro
git checkout --detach 21cdd423480470857e3e074638d412c4d8b745f5
go test ./internal/idempotency -run ^TestConcurrentDuplicateWaitsForOwnerTerminalState$ -count=1
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/idempotency -run ^TestConcurrentDuplicateWaitsForOwnerTerminalState$ -count=1
--- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState (0.01s)
    --- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState/owner_completes (0.00s)
        concurrent_test.go:53: duplicate returned before owner terminal state: status=102 replay=true err=<nil>
    --- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState/owner_aborts (0.00s)
        concurrent_test.go:53: duplicate returned before owner terminal state: status=102 replay=true err=<nil>
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/idempotency	0.029s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/idempotency -run ^TestConcurrentDuplicateWaitsForOwnerTerminalState$ -count=1
--- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState (0.00s)
    --- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState/owner_completes (0.00s)
        concurrent_test.go:53: duplicate returned before owner terminal state: status=102 replay=true err=<nil>
    --- FAIL: TestConcurrentDuplicateWaitsForOwnerTerminalState/owner_aborts (0.00s)
        concurrent_test.go:53: duplicate returned before owner terminal state: status=102 replay=true err=<nil>
FAIL
FAIL	github.com/VanceMichael/go-base-airbridge/internal/idempotency	0.001s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

根因结论必须说明新 execution 的 executing/102 初态、同指纹重复请求在 BeginContext 中如何绕过 phase 判断直接 snapshot/replay，以及 Complete 与 Abort 虽关闭 done 却没有任何等待者监听的完整时序；还要区分成功后应重放最终结果、失败后应重新取得执行权，以及不同租户或不同指纹的边界。使用 TestConcurrentDuplicateWaitsForOwnerTerminalState 的 -race 红测复核，目标仓库代码、测试和配置保持零改动，不得只归因为“并发太快”或实施修复。
