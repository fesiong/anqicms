package provider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/go-pay/gopay"
)

// alipayNotifyTestKey 造一把测试密钥 + 自签证书（支付宝那边是官方签发的公钥证书，
// 验签用的是证书里的公钥，这里自签一份在本地复现同样的形态）。
func alipayNotifyTestKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "alipay-notify-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("生成证书失败: %v", err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// alipaySignedNotify 按支付宝的签名规则签一组通知参数：
// 去掉 sign/sign_type 后按 key 升序拼 k=v&k=v，SHA256withRSA，base64。
func alipaySignedNotify(t *testing.T, key *rsa.PrivateKey, params map[string]string) gopay.BodyMap {
	t.Helper()
	bm := make(gopay.BodyMap, len(params)+2)
	for k, v := range params {
		bm.Set(k, v)
	}
	signData := bm.EncodeAliPaySignParams()
	digest := sha256.Sum256([]byte(signData))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	bm.Set("sign", base64.StdEncoding.EncodeToString(sig))
	bm.Set("sign_type", "RSA2")
	return bm
}

func cloneBodyMap(bm gopay.BodyMap) gopay.BodyMap {
	c := make(gopay.BodyMap, len(bm))
	for k, v := range bm {
		c.Set(k, v)
	}
	return c
}

// TestAlipayNotifyCertVerify 证书模式的异步通知必须能验过，且篡改后必须验不过。
//
// 注意 `alipay.VerifySign` 会就地 Remove("sign")，所以每次都传 BodyMap 副本，
// 否则第二次调用会拿不到签名，得到「验签失败」的假象。
func TestAlipayNotifyCertVerify(t *testing.T) {
	key, certPEM := alipayNotifyTestKey(t)
	bm := alipaySignedNotify(t, key, map[string]string{
		"app_id":       "2021000000000000",
		"out_trade_no": "pay-1",
		"total_amount": "50.00",
		"trade_status": "TRADE_SUCCESS",
	})

	ok, err := VerifyAlipayNotifySign(certPEM, cloneBodyMap(bm))
	if !ok || err != nil {
		t.Fatalf("证书模式验签必须成功：ok=%v err=%v", ok, err)
	}

	// 篡改金额必须验不过（否则谁都敢伪造支付成功通知）
	tampered := cloneBodyMap(bm)
	tampered.Set("total_amount", "0.01")
	if ok, _ := VerifyAlipayNotifySign(certPEM, tampered); ok {
		t.Fatal("金额被篡改后仍然验签通过")
	}

	if _, err := VerifyAlipayNotifySign(nil, bm); err == nil {
		t.Fatal("空证书内容应报错")
	}
}

// TestAlipayNotifyPublicKeyVerify 支付宝后台直接给的裸 base64 公钥（无 PEM 头）
// 要走 VerifySign 分支，不能被当成证书。
func TestAlipayNotifyPublicKeyVerify(t *testing.T) {
	key, _ := alipayNotifyTestKey(t)
	bm := alipaySignedNotify(t, key, map[string]string{
		"app_id":       "2021000000000000",
		"out_trade_no": "pay-2",
		"total_amount": "10.00",
	})
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("序列化公钥失败: %v", err)
	}
	bareBase64 := []byte(base64.StdEncoding.EncodeToString(der))

	ok, err := VerifyAlipayNotifySign(bareBase64, cloneBodyMap(bm))
	if !ok || err != nil {
		t.Fatalf("裸 base64 公钥验签必须成功：ok=%v err=%v", ok, err)
	}
}
