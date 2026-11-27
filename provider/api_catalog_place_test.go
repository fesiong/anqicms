package provider

import "testing"

// 后台运费设置新增的 3 条 place 路由复用了 controller.ApiPlace*，
// 但 catalog 生成器过去只扫 controller/manageController，导致这 3 条端点
// 顶着空 param_source / params=null —— api_schema 看不到 country_code 等参数。
func TestPlaceEndpointsExposeURLParams(t *testing.T) {
	cat, err := buildAPICatalogFromSource()
	if err != nil {
		t.Fatalf("从源码派生端点表失败: %v", err)
	}
	byPath := map[string]EndpointMeta{}
	for _, ep := range cat.Endpoints {
		byPath[ep.Path] = ep
	}

	cases := []struct {
		path   string
		params []string
	}{
		{
			path:   "/system/api/plugin/place/country/states",
			params: []string{"country_code", "country_name"},
		},
		{
			path:   "/system/api/plugin/place/country/cities",
			params: []string{"country_code", "state_code", "country_name", "state_name"},
		},
	}
	for _, c := range cases {
		ep, ok := byPath[c.path]
		if !ok {
			t.Fatalf("缺少端点 %s", c.path)
		}
		if ep.ParamSource != "urlparam" {
			t.Errorf("%s 的 param_source = %q，期望 urlparam", c.path, ep.ParamSource)
		}
		for _, name := range c.params {
			if !hasCatalogParam(ep.Params, name) {
				t.Errorf("%s 缺少参数 %s（实际 params=%v）", c.path, name, ep.Params)
			}
		}
	}

	countries, ok := byPath["/system/api/plugin/place/countries"]
	if !ok {
		t.Fatal("缺少端点 /system/api/plugin/place/countries")
	}
	if countries.ParamSource != "none" {
		t.Errorf("countries 的 param_source = %q，期望 none", countries.ParamSource)
	}
}

func hasCatalogParam(params []ParamMeta, name string) bool {
	for _, p := range params {
		if p.Name == name {
			return true
		}
	}
	return false
}
