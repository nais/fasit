package layout

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPageRefreshInterval(t *testing.T) {
	var buf bytes.Buffer
	if err := Page(Props{RefreshInterval: time.Minute}).Render(&buf); err != nil {
		t.Fatalf("render page: %v", err)
	}

	if !strings.Contains(buf.String(), `<meta http-equiv="refresh" content="60">`) {
		t.Errorf("page did not render a 60-second refresh meta tag: %s", buf.String())
	}
}

func TestPageWithoutRefreshInterval(t *testing.T) {
	var buf bytes.Buffer
	if err := Page(Props{}).Render(&buf); err != nil {
		t.Fatalf("render page: %v", err)
	}

	if strings.Contains(buf.String(), `http-equiv="refresh"`) {
		t.Errorf("page unexpectedly rendered a refresh meta tag: %s", buf.String())
	}
}
