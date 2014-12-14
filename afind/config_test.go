package afind

import (
	"github.com/andaru/afind/utils"
	"testing"
)

func newConfig() Config {
	return Config{RepoMeta: make(map[string]string)}
}

func eq(t *testing.T, exp, val interface{}) bool {
	if exp != val {
		t.Errorf("want %v, got %v", exp, val)
		return false
	}
	return true
}

func neq(t *testing.T, exp, val interface{}) bool {
	if exp == val {
		t.Errorf("don't want both equal to %v", exp)
		return false
	}
	return true
}

func TestConfig(t *testing.T) {
	c := newConfig()
	// test we have a default hostname from the OS or "localhost"
	neq(t, "", c.Host())
	eq(t, c.PortRpc(), utils.DefaultRpcPort)
	c.RepoMeta["host"] = "host123"
	c.RepoMeta["port.rpc"] = defaultPortRpc
	eq(t, "host123", c.Host())
	eq(t, "30800", c.PortRpc())
	neq(t, nil, c.GetTimeoutIndex())
	neq(t, nil, c.GetTimeoutSearch())
	eq(t, false, c.Verbose())
	c.SetVerbose(true)
	eq(t, true, c.Verbose())
}

func TestListenerRpc(t *testing.T) {
	c := newConfig()
	c.RpcBind = "0.0.0.0:0"
	listenah, err := c.ListenerRpc()
	neq(t, listenah.Addr().String(), "0.0.0.0:0")
	neq(t, listenah.Addr().String(), "")
	eq(t, err, nil)
}
