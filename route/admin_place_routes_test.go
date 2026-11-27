package route

import "testing"

// 后台运费设置的地区级联调用的是 /system/api/plugin/place/*，
// 但后端过去只在 /api/place/* 注册了这三个接口，导致后台 404、地区下拉永远“暂无数据”。
// 这里锁真实注册表，防止路由再次缺失。
func TestAdminPlaceRoutesRegistered(t *testing.T) {
	routes := registeredRoutes(t)

	want := []string{
		"GET /system/api/plugin/place/countries",
		"GET /system/api/plugin/place/country/states",
		"GET /system/api/plugin/place/country/cities",
	}
	for _, path := range want {
		if !routes[path] {
			t.Errorf("缺少后台地区路由 %s：运费设置的地区下拉会 404", path)
		}
	}

	// 反向对照：确认 registeredRoutes 不是“什么都返回 true”，否则上面的断言会空转。
	if routes["GET /system/api/plugin/place/not-exists"] {
		t.Error("反向对照失效：不存在的路径不应出现在真实路由表里")
	}
}
