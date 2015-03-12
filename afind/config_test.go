package afind

import (
	"testing"

	"github.com/andaru/afind/utils"
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
	eq(t, "host123", c.Host())
	c.RepoMeta["host"] = "newhost"
	eq(t, "newhost", c.Host())
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
	_ = listenah.Close()
}

func TestIsHostLocal(t *testing.T) {
	c := newConfig()
	hostname := c.Host()
	eq(t, true, c.IsHostLocal(hostname))
	eq(t, true, c.IsHostLocal("localhost"))
	eq(t, true, c.IsHostLocal(""))
	eq(t, false, c.IsHostLocal("__cannot_be_this__"))
}
