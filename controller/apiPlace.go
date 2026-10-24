package controller

import (
	"encoding/json"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

const PlaceApi = "https://places.anqicms.com/v1"

func ApiPlaceCountries(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	resp, err := library.GetURLData(PlaceApi+"/countries", currentSite.System.BaseUrl, 10)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var countries []interface{}
	_ = json.Unmarshal([]byte(resp.Body), &countries)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": countries,
	})
}

func ApiPlaceCountryStates(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	countryCode := ctx.URLParam("country_code")
	countryName := ctx.URLParam("country_name")
	if countryCode == "" && countryName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Country Code or Name is required",
		})
		return
	}
	var resp *library.RequestData
	var err error
	if countryName != "" {
		resp, err = library.GetURLData(PlaceApi+"/countries/states?country_name="+countryName, currentSite.System.BaseUrl, 10)
	} else {
		resp, err = library.GetURLData(PlaceApi+"/countries/"+countryCode+"/states", currentSite.System.BaseUrl, 10)
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var states []interface{}
	_ = json.Unmarshal([]byte(resp.Body), &states)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": states,
	})
}

func ApiPlaceCountryCities(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	countryCode := ctx.URLParam("country_code")
	stateCode := ctx.URLParam("state_code")
	countryName := ctx.URLParam("country_name")
	stateName := ctx.URLParam("state_name")
	if countryCode == "" && countryName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Country Code or Name is required",
		})
		return
	}
	var err error
	var resp *library.RequestData
	if countryName != "" {
		resp, err = library.GetURLData(PlaceApi+"/countries/cities?country_name="+countryName+"&state_name="+stateName, currentSite.System.BaseUrl, 10)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		if stateCode == "" {
			resp, err = library.GetURLData(PlaceApi+"/countries/"+countryCode+"/cities", currentSite.System.BaseUrl, 10)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		} else {
			resp, err = library.GetURLData(PlaceApi+"/countries/"+countryCode+"/states/"+stateCode+"/cities", currentSite.System.BaseUrl, 10)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	}

	var cities []interface{}
	_ = json.Unmarshal([]byte(resp.Body), &cities)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": cities,
	})
}

func ApiPlaceAutocomplete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req map[string]interface{}
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	authSetting := currentSite.GetGoogleAuthSetting()
	if authSetting.PlacesApiKey == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("GooglePlacesApiKeyNotSet"),
		})
	}

	if req["regionCode"] != nil && req["includedRegionCodes"] == nil {
		regionCode, _ := req["regionCode"].(string)
		req["includedRegionCodes"] = []string{regionCode}
	}

	resp, err := library.Request("https://places.googleapis.com/v1/places:autocomplete", &library.Options{
		Timeout: 10,
		Method:  "POST",
		Type:    "json",
		Data:    req,
		Header:  map[string]string{"X-Goog-Api-Key": authSetting.PlacesApiKey},
	})
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var respData map[string]interface{}
	_ = json.Unmarshal([]byte(resp.Body), &respData)
	if respData["error"] != nil {
		message, _ := respData["error"].(map[string]interface{})["message"]
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  message,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": respData,
	})
}
