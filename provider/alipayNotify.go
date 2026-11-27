package provider

import (
	"bytes"
	"errors"
	"strings"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
)

// VerifyAlipayNotifySign 校验支付宝异步通知。
//
// 站点里 PublicCertPath 存的是**公钥证书**（.crt）。证书模式下应当走
// VerifySignWithCert：它用 xpem.DecodePublicKey 直接解析 PEM 块，同时支持
// CERTIFICATE 与 PUBLIC KEY 两种形态；而 VerifySign 的入参约定是「裸 base64 公钥」
// （无头无尾），内部用 FormatAlipayPublicKey 重新包一层 PEM 头再解析。
// 把带 PEM 头的证书内容喂给它，一旦文件换行/带证书链就会被包成畸形的 PUBLIC KEY 块
// （实测出现过 pem.Decode 失败），属于「同一份配置换个文件就静默失效」的隐患。
//
// 所以这里按内容分流：带 "-----BEGIN" 的走 VerifySignWithCert，
// 支付宝后台直接复制的裸 base64 公钥走 VerifySign。
func VerifyAlipayNotifySign(certContent []byte, bm gopay.BodyMap) (bool, error) {
	if len(bytes.TrimSpace(certContent)) == 0 {
		return false, errors.New("alipay notify: 公钥/证书内容为空")
	}
	if bm == nil {
		return false, errors.New("alipay notify: 参数为空")
	}
	if bytes.Contains(certContent, []byte("-----BEGIN")) {
		return alipay.VerifySignWithCert(certContent, bm)
	}
	return alipay.VerifySign(strings.TrimSpace(string(certContent)), bm)
}
