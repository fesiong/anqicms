package manageController

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 任务 16D：后台保存端点的阿里云 endpoint 入站归一化（裁决 R36）
//
// 站长最常见的填法是带 scheme 的「https://dysmsapi.aliyuncs.com」，而适配器
// 裸拼 "https://"+endpoint+"/?" → 拼出 https://https//… 的畸形 URL，要到发请求
// 才炸（DNS 失败被判 transient，站点误以为平台抖动）。
//
// 两道防线（缺一不可）：
//   ① 保存时归一化（写入口）—— 库里从此是干净的 host；
//   ② ApplyDefaults 防御性归一化（读入口）—— 覆盖库里既有的脏值。
// 本文件只钉 ①，② 由 provider/sms 包的测试钉。
// ---------------------------------------------------------------------------

// s13CheckSettingFormEndpointNormalize 断言 PluginSmsSettingForm 在**落库之前**
// 对两个阿里云 endpoint 槽做了归一化。
func s13CheckSettingFormEndpointNormalize(src string) error {
	for _, slot := range []string{"Aliyun.Domestic.Endpoint", "Aliyun.International.Endpoint"} {
		anchor := "sms.NormalizeEndpointHost(req." + slot + ")"
		if !strings.Contains(src, anchor) {
			return errors.New("保存路径未归一化 " + slot + "（缺 " + anchor + "）")
		}
	}
	// 归一化必须发生在写站点内存与落库之前，否则存进去的还是脏值。
	iNormalize := strings.Index(src, "sms.NormalizeEndpointHost(req.Aliyun.Domestic.Endpoint)")
	iAssign := strings.Index(src, "*currentSite.PluginSms = req")
	iSave := strings.Index(src, "SaveSettingValue(")
	if iNormalize < 0 || iAssign < 0 || iSave < 0 {
		return errors.New("源码结构异常：找不到归一化 / 赋值 / 落库三者之一")
	}
	if iNormalize > iAssign {
		return errors.New("归一化发生在写站点内存之后，内存里仍是脏值")
	}
	if iNormalize > iSave {
		return errors.New("归一化发生在落库之后，库里存的是脏值")
	}
	return nil
}

// TestPluginSmsSettingFormNormalizesEndpoint 三段式：真实源码绿 / 删掉归一化红 /
// 恢复绿。
func TestPluginSmsSettingFormNormalizesEndpoint(t *testing.T) {
	real := smsFuncSource(t, "pluginSms.go", "PluginSmsSettingForm")
	if err := s13CheckSettingFormEndpointNormalize(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Log("真实源码：保存前归一化两个阿里云 endpoint 槽 → 判合规（绿）")

	t.Run("反向对照_删掉归一化必红", func(t *testing.T) {
		broken := strings.Replace(real,
			"\treq.Aliyun.Domestic.Endpoint = sms.NormalizeEndpointHost(req.Aliyun.Domestic.Endpoint)\n", "", 1)
		if broken == real {
			t.Fatal("反向对照锚点失效：源码里没找到国内 endpoint 的归一化语句")
		}
		if err := s13CheckSettingFormEndpointNormalize(broken); err == nil {
			t.Fatal("反向对照失败：删掉归一化后护栏仍判绿")
		}
		t.Log("反向对照：删掉归一化 → 判违规（红）")
	})

	t.Run("反向对照_归一化挪到落库之后必红", func(t *testing.T) {
		broken := strings.Replace(real,
			"\treq.Aliyun.Domestic.Endpoint = sms.NormalizeEndpointHost(req.Aliyun.Domestic.Endpoint)\n", "", 1)
		broken = strings.Replace(broken,
			"\t*currentSite.PluginSms = req\n",
			"\t*currentSite.PluginSms = req\n\treq.Aliyun.Domestic.Endpoint = sms.NormalizeEndpointHost(req.Aliyun.Domestic.Endpoint)\n", 1)
		if broken == real {
			t.Fatal("反向对照锚点失效：源码结构变了")
		}
		if err := s13CheckSettingFormEndpointNormalize(broken); err == nil {
			t.Fatal("反向对照失败：归一化挪到写内存之后仍判绿")
		}
		t.Log("反向对照：归一化挪到写内存之后 → 判违规（红）")
	})

	if err := s13CheckSettingFormEndpointNormalize(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
