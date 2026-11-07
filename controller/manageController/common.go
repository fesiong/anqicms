package manageController

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	captcha "github.com/mojocn/base64Captcha"
	"github.com/parnurzeal/gorequest"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

// AdminFileServ 后台系统文件服务，根据请求路径输出 system 目录下的静态文件（支持目录默认 index.html）；同时根据 Site-Id 请求头或 site_id 参数为二级目录站点切换站点上下文。
func AdminFileServ(ctx iris.Context) {
	tmpSiteId := ctx.GetHeader("Site-Id")
	paramSiteId := ctx.URLParam("site_id")
	if tmpSiteId == "" && paramSiteId != "" {
		tmpSiteId = paramSiteId
	}
	if len(tmpSiteId) > 0 {
		siteId, _ := strconv.Atoi(tmpSiteId)
		if siteId > 0 {
			// 只有二级目录安装的站点允许这么操作
			website := provider.GetWebsite(uint(siteId))
			if website != nil && len(website.BaseURI) > 1 {
				ctx.Values().Set("siteId", uint(siteId))
			}
		}
	}

	uri := ctx.RequestPath(false)
	if uri != "/" {
		uriFile := config.ExecPath + strings.TrimLeft(uri, "/")
		// 防止路径遍历: 解析出真实路径后检查是否仍在允许的目录内
		uriFile = filepath.Clean(uriFile)
		basePath := config.ExecPath + "system"
		if !strings.HasPrefix(uriFile, basePath) {
			ctx.StatusCode(403)
			return
		}
		info, err := os.Stat(uriFile)
		if err == nil && !info.IsDir() {
			ctx.ServeFile(uriFile)
			return
		}

		if !strings.Contains(filepath.Base(uri), ".") {
			uriFile = strings.TrimRight(uriFile, "/") + "/index.html"
			_, err = os.Stat(uriFile)
			if err == nil {
				ctx.ServeFile(uriFile)
				return
			}
		}
	}

	ctx.Next()
}

// Version 获取当前系统版本号及是否为试用版。
func Version(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"version": config.Version,
			"trial":   config.Trial,
		},
	})
}

// GetStatisticsSummary 获取统计摘要数据，exact 为 true 时进行精确统计。
//
// 参数说明：
//   - 查询参数 "exact": 是否精确统计，默认为 false。
func GetStatisticsSummary(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	exact := ctx.URLParamBoolDefault("exact", false)
	result := currentSite.GetStatisticsSummary(exact)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// GetStatisticsDashboard 获取统计面板数据。
func GetStatisticsDashboard(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var result = iris.Map{}
	// 登录信息
	var loginLogs []model.AdminLoginLog
	currentSite.DB.Order("id desc").Limit(2).Find(&loginLogs)
	if len(loginLogs) == 2 {
		result["last_login"] = iris.Map{
			"created_time": loginLogs[1].CreatedTime,
			"ip":           loginLogs[1].Ip,
		}
	}
	if len(loginLogs) > 0 {
		result["now_login"] = iris.Map{
			"created_time": loginLogs[0].CreatedTime,
			"ip":           loginLogs[0].Ip,
		}
	}
	// 配置信息
	result["system"] = currentSite.System

	result["version"] = config.Version
	result["trial"] = config.Trial
	// 当前构建的品牌标识。排障时第一件事就是确认「这个包是给谁做的」，
	// 而它只能来自编译期注入，不能在界面上改。
	result["oem"] = config.CurrentOEMID()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	result["memory_usage"] = ms.Alloc

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// versionCheckURL 拼出「检查最新版」的请求地址。
//
// oem 参数不能省：官网用同一个 shopversion.json 服务所有品牌，
// 靠它决定把哪条版本线（以及配套的升级包命名）下发给这个客户端。
func versionCheckURL(brand config.OEMBrand) string {
	return brand.DownloadBase + "/shopversion.json?goos=" + runtime.GOOS +
		"&goarch=" + runtime.GOARCH + "&type=" + config.VersionType +
		"&oem=" + url.QueryEscape(brand.ID)
}

// upgradePackageURL 拼出升级包下载地址。包名里的主程序名取品牌配置，
// 与升级包里实际的文件名同源，否则 isMainBinaryFile 会认不出主程序。
func upgradePackageURL(brand config.OEMBrand, version string) string {
	name := brand.Binary + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if config.VersionType != "" {
		name = brand.Binary + "-" + config.VersionType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	}
	return brand.DownloadBase + "/" + name + "-v" + version + ".zip?oem=" + url.QueryEscape(brand.ID)
}

// CheckVersion 检查当前版本是否为最新版。
func CheckVersion(ctx iris.Context) {
	link := versionCheckURL(config.Brand())
	var lastVersion response.LastVersion
	_, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).Timeout(10 * time.Second).Get(link).EndBytes()
	if errs != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("CheckThatTheVersionIsTheLatestVersion"),
		})
		return
	}

	err := json.Unmarshal(body, &lastVersion)
	if err == nil {
		result := library.VersionCompare(lastVersion.Version, config.Version)
		if result == 1 {
			// 测试
			// 版本有更新
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  ctx.Tr("FoundANewVersion"),
				"data": lastVersion,
			})
			return
		} else if lastVersion.TrialVersion != "" && library.VersionCompare(lastVersion.TrialVersion, config.Version) == 1 {
			lastVersion.Trial = true
			lastVersion.Version = lastVersion.TrialVersion
			lastVersion.Description = lastVersion.TrialDescription
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  ctx.Tr("FoundANewTrialVersion"),
				"data": lastVersion,
			})
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CheckThatTheVersionIsTheLatestVersion"),
	})
}

// VersionUpgrade 执行系统版本升级：下载指定版本的程序压缩包，解压覆盖程序文件（跳过模板等文件）并替换主程序可执行文件。
//
// 参数说明：
//   - 请求体 "version": 目标版本号。
//   - 请求体 "trial": 是否升级为试用版本。
//   - 请求体 "trial_version": 试用版本号，trial 为 true 时使用。
func VersionUpgrade(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var lastVersion request.VersionData
	if err := ctx.ReadJSON(&lastVersion); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 当前主程序的真实路径必须在动手之前确定，而且要归一化：
	// 上一轮升级把 anqicms rename 成 anqicms.old 之后，Linux 会把正在运行的
	// /proc/self/exe 同步改成 anqicms.old，此时再升一次就会把备份当成主程序，
	// 备份出 anqicms.old.old，真正的 anqicms 一次都没被替换，界面却报成功。
	rawExec, err := os.Executable()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	execPath, runningOld := provider.NormalizeExecPath(rawExec)
	if runningOld || execPath == "" {
		log.Printf("Upgrade aborted: running backup binary %q", rawExec)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UpgradeRequiresRestart", filepath.Base(rawExec), filepath.Base(execPath)),
		})
		return
	}
	exec := filepath.Base(execPath)
	tmpExecPath := execPath + ".new"

	var version = lastVersion.Version
	if lastVersion.Trial {
		version = lastVersion.TrialVersion
	}
	// 下载压缩包
	// 包名由官网按 oem 参数决定，这里只按约定拼出请求地址；
	// 主程序名取自品牌配置，与升级包里实际的文件名同源。
	brand := config.Brand()
	link := upgradePackageURL(brand, version)
	// 最长等待10分钟
	resp, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).Timeout(20 * time.Minute).Get(link).EndBytes()
	if errs != nil || resp.StatusCode != 200 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	// 将文件写入
	tmpFile := config.ExecPath + filepath.Base(link)
	err = os.WriteFile(tmpFile, body, os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	// 解压
	zipReader, err := zip.OpenReader(tmpFile)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	defer func() {
		zipReader.Close()

		// 删除压缩包
		os.Remove(tmpFile)
	}()

	// 删除 system
	_ = os.RemoveAll(config.ExecPath + "system")

	var errorFiles []string

	mainBinaryFound := false

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// 模板文件不更新
		if strings.Contains(f.Name, "static/") ||
			strings.Contains(f.Name, "template/") ||
			strings.Contains(f.Name, ".sh") ||
			strings.Contains(f.Name, ".bat") ||
			strings.Contains(f.Name, "favicon.ico") {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			continue
		}
		realName := filepath.Join(config.ExecPath, f.Name)
		isMainBinary := isMainBinaryFile(f.Name, brand, exec)
		if isMainBinary {
			mainBinaryFound = true
			// 先以临时文件存储
			realName = tmpExecPath
		}
		_ = os.MkdirAll(filepath.Dir(realName), os.ModePerm)
		newFile, err := os.Create(realName)
		if err != nil {
			reader.Close()
			errorFiles = append(errorFiles, realName)
			continue
		}
		_, err = io.Copy(newFile, reader)
		if err != nil {
			reader.Close()
			newFile.Close()
			errorFiles = append(errorFiles, realName)
			continue
		}

		reader.Close()
		_ = newFile.Close()
		// 对于可执行文件，需要赋予可执行权限，可执行文件有：主程序,cwebp
		if isMainBinary || strings.HasPrefix(f.Name, "cwebp_") {
			_ = os.Chmod(realName, os.ModePerm)
		}
	}
	// 包里没有主程序，几乎总是品牌配置的 binary 名与实际包不一致。
	// 原实现会让后续 rename 失败、回滚旧文件，然后照样回「升级成功」——
	// 界面显示已是最新、跑的仍是旧版本，是本次品牌化最容易踩的静默坑。
	if !mainBinaryFound {
		log.Printf("Upgrade aborted: no main binary %q / %q in %s", brand.Binary, exec, link)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	// 更换主程序。失败必须报错：升级最坏的状态不是失败，是「界面说成功、跑的还是旧程序」。
	if err = swapExecutable(tmpExecPath, execPath); err != nil {
		log.Printf("Upgrade swap executable failed: %v", err)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed") + ": " + err.Error(),
		})
		return
	}
	if len(errorFiles) > 1 {
		log.Println("Upgrade error files: ", errorFiles)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdatePartiallyFailed", errorFiles),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSystemVersion", config.Version, lastVersion.Version))
	msg := ctx.Tr("TheVersionHasBeenUpgraded")
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
	})
}

// swapExecutable 用 newPath 处的文件替换 targetPath，旧文件备份为 targetPath+".old"。
//
// 三条硬规矩（每一条都对应一次真实事故）：
//  1. 备份路径永远由目标文件自身推出（同目录 + ".old"），绝不拼工作目录 ——
//     config.ExecPath 与主程序所在目录在 go_build、软链、自定义安装下并不相同；
//  2. 前置校验要报出具体缺失路径，否则线上只能靠猜；
//  3. 替换失败必须返回错误。凡是「报成功但没换掉」的路径都是埋雷。
func swapExecutable(newPath, targetPath string) error {
	oldPath := targetPath + ".old"

	if _, err := os.Stat(newPath); err != nil {
		return fmt.Errorf("新程序不存在：%s", newPath)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("当前主程序不存在：%s", targetPath)
	}
	if info.IsDir() {
		return fmt.Errorf("当前主程序路径是目录：%s", targetPath)
	}
	dirInfo, err := os.Stat(filepath.Dir(oldPath))
	if err != nil {
		return fmt.Errorf("备份目录不可用：%s（%v）", filepath.Dir(oldPath), err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("备份目录不是目录：%s", filepath.Dir(oldPath))
	}

	// 先清掉历史版本留下的 xxx.old.old / xxx.old.old.old，
	// 再清本轮备份位置：Windows 上 rename 到已存在的目标是直接失败的。
	cleanupNestedBackups(targetPath)
	_ = os.Remove(oldPath)

	if err = moveExecutable(targetPath, oldPath); err != nil {
		return fmt.Errorf("备份旧程序失败：%s → %s（%v）", targetPath, oldPath, err)
	}
	if err = moveExecutable(newPath, targetPath); err != nil {
		// 换不上去就把备份退回原位，别把站点留在「没有主程序」的状态
		if rerr := moveExecutable(oldPath, targetPath); rerr != nil {
			return fmt.Errorf("替换主程序失败且回滚失败：%s → %s（%v / 回滚 %v）", newPath, targetPath, err, rerr)
		}
		return fmt.Errorf("替换主程序失败：%s → %s（%v）", newPath, targetPath, err)
	}
	_ = os.Chmod(targetPath, os.ModePerm)
	// 备份保留到下一次升级：新程序起不来时还能手工换回去。
	log.Printf("Upgrade: main binary replaced, backup kept at %s", oldPath)
	return nil
}

// moveExecutable 把 src 搬到 dst，用于替换主程序。
//
// 优先 rename：同目录内是原子操作，也是唯一能安全「覆盖正在运行的程序」的方式
// （直接 open 正在执行的文件写会得到 ETXTBSY）。rename 失败时依次退化为
// 「删掉旧 inode 再 rename」「复制到临时文件再改名」——跨设备、目标被占用等场景下
// 这两步能救回来，而复制走的是新 inode，不会碰到正在运行的那个。
func moveExecutable(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	_ = os.Remove(dst)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyExecutable(src, dst); err != nil {
		return err
	}
	// 复制成功才删源，保持"搬走"语义；删失败不影响结果。
	_ = os.Remove(src)
	return os.Chmod(dst, os.ModePerm)
}

// copyExecutable 先写 dst+".tmp" 再改名覆盖，避免写到一半把目标截断成半个可执行文件。
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err = os.Rename(tmp, dst); err != nil {
		// Windows 上目标存在时改名会失败，删掉再试一次
		if rmErr := os.Remove(dst); rmErr != nil && !os.IsNotExist(rmErr) {
			_ = os.Remove(tmp)
			return err
		}
		if err = os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

// cleanupNestedBackups 清掉「升级后未重启」留下的嵌套备份：xxx.old.old、xxx.old.old.old…
//
// 那些文件是旧实现的产物（每升一次就把上一轮的备份再备份一次），既占磁盘又会误导
// 运维以为有可用备份。只处理 .old 的重复后缀，不碰当前主程序和单层备份。
func cleanupNestedBackups(targetPath string) {
	for depth := 2; depth <= 8; depth++ {
		path := targetPath + strings.Repeat(".old", depth)
		if err := os.Remove(path); err == nil {
			log.Printf("Upgrade: removed nested backup %s", path)
		}
	}
}

// isMainBinaryFile 判断升级包里的这个条目是不是主程序。
//
// 依据两条，满足其一即可：
//   - 与品牌配置的 binary 同名：自己发布的包（每个 OEM 的主程序名不同）
//   - 与当前运行中的可执行文件同名：客户改过文件名的情况
//
// 早期实现硬编码 anqicms，品牌化之后必须跟着品牌走 —— 只比较 anqicms 的话，
// OEM 包的升级会找不到主程序，而后续回滚逻辑会让它「成功」地什么也没换。
func isMainBinaryFile(name string, brand config.OEMBrand, running string) bool {
	base := strings.TrimSuffix(filepath.Base(name), ".exe")
	if base == strings.TrimSuffix(brand.Binary, ".exe") && base != "" {
		return true
	}
	running = strings.TrimSuffix(running, ".exe")
	return running != "" && base == running
}

// GenerateCaptcha 生成验证码
func GenerateCaptcha(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	safeSetting := currentSite.Safe
	// 验证码是一次性凭据，绝不能被任何一层缓存回放：
	// 命中缓存会让前端拿到已作废的 captcha_id，表现为"点了图片不换"或验证码永远校验失败。
	ctx.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	ctx.Header("Pragma", "no-cache")                       // HTTP 1.0 兼容
	ctx.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT") // 过期时间设为过去
	if safeSetting == nil || safeSetting.AdminCaptchaOff == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"captcha_off": true,
			},
		})
		return
	}

	var driver = controller.NewDriver().ConvertFonts()
	c := captcha.NewCaptcha(driver, controller.Store)
	id, content, answer := c.Driver.GenerateIdQuestionAnswer()
	item, _ := c.Driver.DrawCaptcha(content)
	c.Store.Set(id, answer)

	bs64 := item.EncodeB64string()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"captcha_off": false,
			"captcha_id":  id,
			"captcha":     bs64,
		},
	})
}
