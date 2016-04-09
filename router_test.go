package router_test

import (
	"testing"
	"strconv"
	"gopkg.in/orivil/router.v0"
)

var r = router.NewRouter()

type testData struct {
	// for test function Add()
	addRoute  string
	addID     int
	returnID  int

	// for test function Match()
	matchPath string
	matchID   int
	param     router.Param
	matched   bool
}

var data = []testData{
	{
		addRoute: "/user/password",
		addID: 1,
		returnID: 0,

		matchPath: "/user/password",
		matchID: 1,
		param: nil,
		matched: true,
	},
	{
		addRoute: "/user/::password",
		addID: 2,
		returnID: 0,

		matchPath: "/user/123456",
		matchID: 2,
		param: router.Param{"password": "123456"},
		matched: true,
	},
	{
		addRoute: "/user",
		addID: 3,
		returnID: 0,

		matchPath: "/user",
		matchID: 3,
		param: nil,
		matched: true,
	},
	{
		addRoute: "/::user",
		addID: 4,
		returnID: 0,

		matchPath: "/foobar",
		matchID: 4,
		param: router.Param{"user": "foobar"},
		matched: true,
	},
	{
		addRoute: "/user/password/address",
		addID: 5,
		returnID: 0,

		matchPath: "/user/password/address",
		matchID: 5,
		param: nil,
		matched: true,
	},
	{
		addRoute: "/::user/::password/::address",
		addID: 6,
		returnID: 0,

		matchPath: "/jaychou/123456/china",
		matchID: 6,
		param: router.Param{
			"user": "jaychou",
			"password": "123456",
			"address": "china",
		},
		matched: true,
	},
}

func checkID(expect int, got int, t *testing.T) {
	if expect != got {
		t.Errorf("expect: %d, got: %d\n", expect, got)
	}
}

func checkParam(expect router.Param, got router.Param, t *testing.T) {
	if expect != nil || got != nil {
		if expect != nil && got != nil {
			if len(expect) != len(got) {
				t.Errorf("expect: %v, got: %v\n", expect, got)
			}
			for key, value := range expect {
				if got[key] != value {
					t.Errorf("expect: %v, got: %v\n", expect, got)
					break
				}
			}
		} else {
			t.Errorf("expect: %v, got: %v\n", expect, got)
		}
	}
}

func checkMatched(expect bool, got bool, t *testing.T) {
	if expect != got {
		t.Errorf("expect: %d, got: %d\n", expect, got)
	}
}

func TestAdd(t *testing.T) {
	for _, d := range data {
		returnID, err := r.Add(d.addRoute, d.addID)
		if err != nil {
			t.Fatal(err)
		}
		checkID(d.returnID, returnID, t)
	}
}

func TestMatch(t *testing.T) {
	for _, d := range data {
		id, params, ok := r.Match(d.matchPath)
		checkID(d.matchID, id, t)
		checkMatched(d.matched, ok, t)
		checkParam(d.param, params, t)
	}
}

func BenchmarkAddId(b *testing.B) {
	// 随机添加带参数的路由
	for i := 1; i < b.N; i++ {
		path := "/" + strconv.Itoa(i) +
		"/::" + strconv.Itoa(i + 1) +
		"/::" + strconv.Itoa(i + 2) +
		"/::" + strconv.Itoa(i + 3)
		r.Add(path, i)
	}
}

func BenchmarkMatch(b *testing.B) {
	// 随机匹配 path
	for i := 1; i < b.N; i++ {
		path := "/" + strconv.Itoa(i) +
		"/" + strconv.Itoa(i + 1) +
		"/" + strconv.Itoa(i + 2) +
		"/" + strconv.Itoa(i + 3)
		r.Match(path)
	}
}

// 测试字符串转换速度
func BenchmarkStrconv(b *testing.B) {
	var str string
	for i := 1; i < b.N; i++ {
		str = "/" + strconv.Itoa(i) +
		"/" + strconv.Itoa(i + 1) +
		"/" + strconv.Itoa(i + 2) +
		"/" + strconv.Itoa(i + 3)
	}
	if len(str) > 0 {} // the str must be used
}

// env: go1.5.2	windows10
//
//	BenchmarkAddId-8          500000              3108 ns/op
//	BenchmarkMatch-8         1000000              1688 ns/op
//	BenchmarkStrconv-8       3000000               411 ns/op

// env: go1.6 Ubuntu14.04
//
// BenchmarkAddId-8  	  500000	      3551 ns/op
// BenchmarkMatch-8  	 1000000	      1259 ns/op
// BenchmarkStrconv-8	 5000000	       346 ns/op
