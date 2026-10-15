package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

func TestEveryKeyLayoutRoundTripsThroughSaveAndLoad(t *testing.T) {
	for n := 1; n <= keys.MaxKeys; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			c := Defaults()
			c.Keys.Enabled, c.Keys.Layout, c.Keys.Scratch = true, keys.LayoutName(n), "ALT_R"
			list := make([]keys.Key, n)
			for i := range list {
				list[i] = keys.Key(fmt.Sprintf("F%d", i+1))
			}
			switch n {
			case 1:
				c.Keys.K1 = list
			case 2:
				c.Keys.K2 = list
			case 3:
				c.Keys.K3 = list
			case 4:
				c.Keys.K4 = list
			case 5:
				c.Keys.K5 = list
			case 6:
				c.Keys.K6 = list
			case 7:
				c.Keys.K7 = list
			case 8:
				c.Keys.K8 = list
			case 9:
				c.Keys.K9 = list
			case 10:
				c.Keys.K10 = list
			}
			if err := c.Validate([]string{"neutral-light"}); err != nil {
				t.Fatal(err)
			}
			if err := Save(p, c); err != nil {
				t.Fatal(err)
			}
			got, err := Load(p)
			if err != nil || !reflect.DeepEqual(got, c) || !reflect.DeepEqual(got.Keys.Lanes(n), list) {
				t.Fatalf("got %+v, %v", got.Keys, err)
			}
		})
	}
}

func TestValidateReportsTheFieldOfEveryLayout(t *testing.T) {
	for n := 1; n <= keys.MaxKeys; n++ {
		c := Defaults()
		bad := append(c.Keys.Lanes(n), "Z")
		switch n {
		case 1:
			c.Keys.K1 = bad
		case 2:
			c.Keys.K2 = bad
		case 3:
			c.Keys.K3 = bad
		case 4:
			c.Keys.K4 = bad
		case 5:
			c.Keys.K5 = bad
		case 6:
			c.Keys.K6 = bad
		case 7:
			c.Keys.K7 = bad
		case 8:
			c.Keys.K8 = bad
		case 9:
			c.Keys.K9 = bad
		case 10:
			c.Keys.K10 = bad
		}
		err := c.Validate([]string{"neutral-light"})
		fe, ok := err.(*FieldError)
		if !ok || fe.Field != fmt.Sprintf("keys.k%d", n) {
			t.Errorf("%dk: err = %v", n, err)
		}
	}
}

func TestAV010FileWithOnlyK4AndK7KeepsWorking(t *testing.T) {
	got, err := Load(write(t, `{"version":1,"port":7878,"keys":{"enabled":true,"layout":"auto","k4":["Q","W","E","R"],"k7":["A","S","D","SPACE","J","K","L"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := got.Validate([]string{"neutral-light"}); err != nil {
		t.Fatal(err)
	}
	if !got.Keys.Enabled || got.Keys.Layout != "auto" || got.Keys.K4[0] != "Q" || len(got.Keys.K10) != 10 {
		t.Fatalf("keys = %+v", got.Keys)
	}
}

func TestSaveKeepsUnknownKeysInsideTheKeysSection(t *testing.T) {
	p := write(t, `{"keys":{"enabled":false,"future_option":42}}`)
	if err := Save(p, Defaults()); err != nil {
		t.Fatal(err)
	}
	keysObj, _ := readFileObject(t, p)["keys"].(map[string]any)
	if keysObj["future_option"] != float64(42) || keysObj["k10"] == nil || keysObj["k1"] == nil {
		t.Fatalf("keys section = %v", keysObj)
	}
}

func TestValidateEveryLayoutAndScratchCombination(t *testing.T) {
	themes := []string{"neutral-light"}
	layouts := []string{"auto"}
	for n := 1; n <= keys.MaxKeys; n++ {
		layouts = append(layouts, keys.LayoutName(n))
	}
	for _, layout := range layouts {
		c := Defaults()
		c.Keys.Layout, c.Keys.Scratch = layout, "F12"
		if err := c.Validate(themes); err != nil {
			t.Errorf("%s with a free scratch key: %v", layout, err)
		}
		c.Keys.Scratch = "A"
		if layout == "1k" {
			c.Keys.Scratch = "SPACE"
		}
		err := c.Validate(themes)
		fe, ok := err.(*FieldError)
		if !ok || fe.Field != "keys.scratch" || fe.Msg != keys.ScratchClashMsg {
			t.Errorf("%s with a clashing scratch key: %v", layout, err)
		}
		c.Keys.Scratch = " "
		if fe, ok := c.Validate(themes).(*FieldError); !ok || fe.Field != "keys.scratch" {
			t.Errorf("%s with a blank scratch key accepted", layout)
		}
	}
}

func TestLoadedConfigWithoutScratchKeepsWorking(t *testing.T) {
	got, err := Load(write(t, `{"keys":{"layout":"7k"}}`))
	if err != nil || got.Keys.ScratchKey() != keys.DefaultScratch || got.Validate([]string{"neutral-light"}) != nil {
		t.Fatalf("got %+v, %v", got.Keys, err)
	}
}

func TestV010ConfigWithShiftInAK7ListLoadsValidatesAndSaves(t *testing.T) {
	for _, layout := range []string{"auto", "7k"} {
		p := write(t, `{"version":1,"keys":{"enabled":true,"layout":"`+layout+`","k7":["SHIFT_L","S","D","SPACE","J","K","L"]}}`)
		got, err := Load(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := got.Validate([]string{"neutral-light"}); err != nil {
			t.Fatalf("%s: %v", layout, err)
		}
		if err := Save(p, got); err != nil {
			t.Fatal(err)
		}
		if _, has := readFileObject(t, p)["keys"].(map[string]any)["scratch"]; has {
			t.Errorf("%s: saving wrote a scratch key that the user never set", layout)
		}
		again, err := Load(p)
		if err != nil || again.Validate([]string{"neutral-light"}) != nil || again.Keys.K7[0] != "SHIFT_L" {
			t.Fatalf("%s: reload = %+v, %v", layout, again.Keys, err)
		}
	}
}
