package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- IsCommandReadOnly ---

func TestIsCommandReadOnly_EmptyCommand(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly(""))
}

func TestIsCommandReadOnly_SimpleLS(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("ls -la"))
}

func TestIsCommandReadOnly_LSWithFlags(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("ls -l -a -h --color"))
}

func TestIsCommandReadOnly_CatFile(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("cat -n file.txt"))
}

func TestIsCommandReadOnly_GrepSearch(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("grep -rn pattern src/"))
}

func TestIsCommandReadOnly_RipgrepSearch(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("rg -i --type go pattern"))
}

func TestIsCommandReadOnly_HeadTail(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("head -n 50 file.txt"))
	assert.True(t, agentic.IsCommandReadOnly("tail -n 20 log.txt"))
}

func TestIsCommandReadOnly_GitStatus(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("git status"))
}

func TestIsCommandReadOnly_GitLog(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("git log --oneline -10"))
}

func TestIsCommandReadOnly_GitDiff(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("git diff HEAD"))
}

func TestIsCommandReadOnly_GitBranch(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("git branch"))
}

func TestIsCommandReadOnly_GitPush_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("git push origin main"))
}

func TestIsCommandReadOnly_GitCommit_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("git commit -m 'msg'"))
}

func TestIsCommandReadOnly_GitReset_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("git reset --hard"))
}

func TestIsCommandReadOnly_DockerPS(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("docker ps -a"))
}

func TestIsCommandReadOnly_DockerLogs(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("docker logs mycontainer"))
}

func TestIsCommandReadOnly_DockerRun_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("docker run ubuntu"))
}

func TestIsCommandReadOnly_KubectlGet(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("kubectl get pods -n agenthub"))
}

func TestIsCommandReadOnly_KubectlDescribe(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("kubectl describe pod my-pod"))
}

func TestIsCommandReadOnly_KubectlDelete_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("kubectl delete pod my-pod"))
}

func TestIsCommandReadOnly_KubectlApply_NotReadOnly(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("kubectl apply -f deploy.yaml"))
}

func TestIsCommandReadOnly_PipeBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("ls | grep foo"))
}

func TestIsCommandReadOnly_ChainBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("ls && rm -rf /"))
}

func TestIsCommandReadOnly_SemicolonBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("ls; rm -rf /"))
}

func TestIsCommandReadOnly_BacktickBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("echo `whoami`"))
}

func TestIsCommandReadOnly_SubshellBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("echo $(whoami)"))
}

func TestIsCommandReadOnly_RedirectBlocked(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("ls > output.txt"))
}

func TestIsCommandReadOnly_CodeExec_Python(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("python script.py"))
}

func TestIsCommandReadOnly_CodeExec_Node(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("node index.js"))
}

func TestIsCommandReadOnly_CodeExec_Bash(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("bash script.sh"))
}

func TestIsCommandReadOnly_CodeExec_Sudo(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("sudo ls"))
}

func TestIsCommandReadOnly_CodeExec_SSH(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("ssh user@host"))
}

func TestIsCommandReadOnly_CodeExec_Curl(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("curl https://example.com"))
}

func TestIsCommandReadOnly_UnknownCommand(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("myunknowntool --foo"))
}

func TestIsCommandReadOnly_Pwd(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("pwd"))
}

func TestIsCommandReadOnly_Whoami(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("whoami"))
}

func TestIsCommandReadOnly_Wc(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("wc -l file.txt"))
}

func TestIsCommandReadOnly_Find(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("find . -name '*.go' -type f"))
}

func TestIsCommandReadOnly_Diff(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("diff -u file1.txt file2.txt"))
}

func TestIsCommandReadOnly_JQ(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("jq -r '.name' data.json"))
}

func TestIsCommandReadOnly_Tree(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("tree -L 2 -d"))
}

func TestIsCommandReadOnly_Df(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("df -h"))
}

func TestIsCommandReadOnly_Du(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("du -sh /tmp"))
}

func TestIsCommandReadOnly_AbsolutePathCommand(t *testing.T) {
	assert.True(t, agentic.IsCommandReadOnly("/usr/bin/ls -la"))
}

func TestIsCommandReadOnly_GitNoSubcommand(t *testing.T) {
	assert.False(t, agentic.IsCommandReadOnly("git"))
}

// --- ClassifyCommand ---

func TestClassifyCommand_Safe(t *testing.T) {
	assert.Equal(t, agentic.CommandSafe, agentic.ClassifyCommand("ls -la"))
}

func TestClassifyCommand_NeedsConfirm(t *testing.T) {
	assert.Equal(t, agentic.CommandNeedsConfirm, agentic.ClassifyCommand("npm install"))
}

func TestClassifyCommand_Dangerous_RmRF(t *testing.T) {
	assert.Equal(t, agentic.CommandDangerous, agentic.ClassifyCommand("rm -rf /tmp/stuff"))
}

func TestClassifyCommand_Dangerous_GitForce(t *testing.T) {
	assert.Equal(t, agentic.CommandDangerous, agentic.ClassifyCommand("git push --force"))
}

func TestClassifyCommand_Dangerous_DropTable(t *testing.T) {
	assert.Equal(t, agentic.CommandDangerous, agentic.ClassifyCommand("echo 'DROP TABLE users'"))
}

func TestClassifyCommand_Dangerous_Sudo(t *testing.T) {
	assert.Equal(t, agentic.CommandDangerous, agentic.ClassifyCommand("sudo rm -rf /"))
}

func TestClassifyCommand_Dangerous_KillAll(t *testing.T) {
	assert.Equal(t, agentic.CommandDangerous, agentic.ClassifyCommand("killall myprocess"))
}

// --- splitCommandParts ---

func TestSplitCommandParts_QuotedStrings(t *testing.T) {
	// Verify that quoted strings are kept together.
	result := agentic.IsCommandReadOnly("grep -rn 'hello world' src/")
	assert.True(t, result)
}

func TestSplitCommandParts_DoubleQuotedStrings(t *testing.T) {
	result := agentic.IsCommandReadOnly(`grep -rn "hello world" src/`)
	assert.True(t, result)
}
