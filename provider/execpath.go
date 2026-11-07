package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kandaoni.com/anqicms/config"
)

// deletedSuffix 是 Linux 内核给 /proc/self/exe（即 os.Executable()）加的标记：
// 进程所在的 inode 被 unlink 之后，返回值会变成 "/opt/app/xlspider.old (deleted)"，
// 末尾那截不是真实文件名，拿它去 Stat / rename 必然失败。
const deletedSuffix = " (deleted)"

// NormalizeExecPath 把 os.Executable() 的原始返回值还原成程序「正常」的安装路径，
// 第二个返回值表示当前进程是从备份程序（xxx.old）启动的。
//
// 为什么必须归一化：升级过程会把主程序 rename 成 xxx.old，Linux 下内核会把正在运行的
// 进程的 /proc/self/exe 同步改成新名字，旧备份被删掉时还会追加上面那个 (deleted) 标记。
// 于是「升级完没重启就再点一次升级」时，os.Executable() 拿到的是 xxx.old，
// 若直接当主程序用，就会把 xxx.old 再备份成 xxx.old.old，真正要替换的 xxx 一次都没被碰过，
// 而界面照样提示升级成功 —— 越升越套娃，版本却原地不动。
func NormalizeExecPath(raw string) (string, bool) {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), deletedSuffix)
	if raw == "" {
		return "", false
	}
	dir, base := filepath.Split(raw)
	wasOld := false
	// 一层不够：已经套过娃的机器上是 xxx.old.old.old，同样要折叠回 xxx。
	for strings.HasSuffix(base, ".old") {
		base = strings.TrimSuffix(base, ".old")
		wasOld = true
	}
	if base == "" {
		return "", wasOld
	}
	if dir == "" {
		return base, wasOld
	}
	return filepath.Join(dir, base), wasOld
}

// resolveRestartTarget 决定重新拉起进程时该执行哪个文件。
//
// 升级成功后 /proc/self/exe 往往指向已被删除或改名的 inode，直接 exec 会失败并把
// 服务留在「退出后没人拉起」的状态，所以先按归一化路径找，找不到再回落到同目录下的
// 品牌主程序名（客户把 binary 改成别的名字时第一候选依然有效）。
func resolveRestartTarget(self string) (string, error) {
	target, _ := NormalizeExecPath(self)
	if target == "" {
		return "", fmt.Errorf("无法解析当前程序路径：%q", self)
	}
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	dir := filepath.Dir(target)
	if binary := config.Brand().Binary; binary != "" {
		candidate := filepath.Join(dir, binary)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("找不到可执行的主程序：%s 不存在", target)
}
