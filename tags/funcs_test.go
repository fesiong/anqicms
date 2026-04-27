package tags

import (
	"testing"
	"time"
)

func TestTimestampToDate(t *testing.T) {
	t2 := time.Now().AddDate(0, 1, 0).Unix()
	add := 0
	nowt := time.Now().Unix()
	var df string
	for {
		if t2 > nowt {
			df = ""
			t2 -= 86400
		} else if add < 600 {
			df = ""
			t2 -= 10
			add += 10
		} else if add < 3600 {
			df = "minute"
			t2 -= 60
			add += 60
		} else if add < 86400 {
			df = "hour"
			t2 -= 3600
			add += 3600
		} else if add < 86400*60 {
			df = "month"
			t2 -= 86400
			add += 86400
		} else if add < 86400*30*37 {
			df = "year"
			t2 -= 86400 * 30
			add += 86400 * 30
		} else {
			break
		}
		res1 := TimestampToDate(t2, "friendly")
		res2 := TimestampToDate(t2, "friendly", "en")
		res3 := TimestampToDate(t2, "diff", df)
		t.Log(res1, res2, res3)
	}
}
