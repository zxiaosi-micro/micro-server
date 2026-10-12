// secretcheck · 密钥出库核查（S11-05，NFR-SEC-008：仓库内无任何真实密钥/强密码）。
//
// 规则（以 git 实证为准，不做模式字符串比对）：
//
//	S1 危险文件：.env / keys.json / *.key / *.pem 被 Git 跟踪 → FAIL
//	   （豁免：keys.example/ 下的 sample_*/*_public* 公钥样例——有意入库供对接参考）
//	S2 私钥块：已跟踪文本文件含 PRIVATE KEY PEM 头 → FAIL
//	S3 .env 忽略实证：git check-ignore <repo>/.env 必须命中
//	S4 keygen 输出忽略实证：含 deploy/conf/keys 目录的仓，check-ignore 必须命中
//	   （真实密钥目录不入库；keys.json 元数据随目录同忽略）
//
// 用法（micro-server 根）：go run ./tools/secretcheck [-repo ..]（默认扫 workspace 全部子仓）
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var failN int

func fail(format string, a ...any) {
	failN++
	fmt.Printf("FAIL  "+format+"\n", a...)
}

func ok(format string, a ...any) {
	fmt.Printf("  · "+format+"\n", a...)
}

func main() {
	root := flag.String("repo", "..", "workspace 根（下含各子仓）")
	flag.Parse()

	entries, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "secretcheck: 读 %s: %v\n", *root, err)
		os.Exit(1)
	}

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		repo := filepath.Join(*root, e.Name())
		if st, err := os.Stat(filepath.Join(repo, ".git")); err != nil || !st.IsDir() && !st.Mode().IsRegular() {
			continue // 非仓库目录
		}
		checkRepo(repo)
	}

	fmt.Printf("\nsecretcheck: %d FAIL\n", failN)
	if failN > 0 {
		os.Exit(1)
	}
}

// isSamplePublicKey 公钥样例豁免：keys.example/ 下 sample_*/*_public*（有意入库，只含公钥）。
func isSamplePublicKey(f string) bool {
	if !strings.Contains(filepath.ToSlash(f), "keys.example/") {
		return false
	}
	base := filepath.Base(f)
	return strings.HasPrefix(base, "sample_") || strings.Contains(base, "public")
}

// checkRepo 单仓检查。
func checkRepo(repo string) {
	name := filepath.Base(repo)
	tracked := gitLs(repo)

	// S1 危险文件不得被 Git 跟踪（公钥样例豁免）
	bad := 0
	for _, f := range tracked {
		f = strings.TrimSpace(f)
		base := filepath.Base(f)
		danger := base == ".env" || base == "keys.json" ||
			strings.HasSuffix(base, ".key") || strings.HasSuffix(base, ".pem") ||
			strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx")
		if danger && !isSamplePublicKey(f) {
			fail("[%s/S1] 危险文件被跟踪: %s", name, f)
			bad++
		}
	}
	if bad == 0 {
		ok("[%s/S1] 危险文件（.env/*.key/*.pem/keys.json）零跟踪（公钥样例除外）", name)
	}

	// S2 已跟踪文本文件无私钥块
	leaked := 0
	for _, f := range tracked {
		f = strings.TrimSpace(f)
		if isTextish(f) && hasPrivateKeyBlock(filepath.Join(repo, f)) {
			fail("[%s/S2] 私钥块特征: %s", name, f)
			leaked++
		}
	}
	if leaked == 0 {
		ok("[%s/S2] 已跟踪文件无私钥块特征", name)
	}

	// S3 .env 必须被 ignore 实证
	if checkIgnore(repo, ".env") {
		ok("[%s/S3] .env 已忽略（.env.example 例外）", name)
	} else {
		fail("[%s/S3] .gitignore 未忽略 .env（check-ignore 未命中）", name)
	}

	// S4 keygen 输出目录（存在才有此检查）：真实密钥必须被 ignore 实证
	keysDir := filepath.Join(repo, "deploy", "conf", "keys")
	if _, err := os.Stat(keysDir); err == nil {
		if checkIgnore(repo, "deploy/conf/keys/data_key.txt") {
			ok("[%s/S4] keygen 输出目录已被忽略（deploy/conf/keys/）", name)
		} else {
			fail("[%s/S4] deploy/conf/keys/ 未被忽略（真实密钥会入库）", name)
		}
	}
}

func gitLs(repo string) []string {
	out, err := exec.Command("git", "-C", repo, "ls-files").Output()
	if err != nil {
		return nil
	}
	return strings.Split(string(out), "\n")
}

// checkIgnore git check-ignore 实证（命中=被忽略；非零退出=未忽略）。
func checkIgnore(repo, rel string) bool {
	return exec.Command("git", "-C", repo, "check-ignore", "-q", rel).Run() == nil
}

func isTextish(f string) bool {
	for _, ext := range []string{".go", ".yaml", ".yml", ".json", ".md", ".sh", ".ps1", ".sql", ".ts", ".tsx", ".txt", ".example"} {
		if strings.HasSuffix(f, ext) {
			return true
		}
	}
	return false
}

// hasPrivateKeyBlock 私钥 PEM 特征（只判头行，不读内容）。
func hasPrivateKeyBlock(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.Contains(strings.TrimSpace(sc.Text()), "PRIVATE KEY-----") {
			return true
		}
	}
	return false
}
