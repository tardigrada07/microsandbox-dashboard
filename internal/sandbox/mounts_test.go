package sandbox

import "testing"

func TestParseStoredMounts(t *testing.T) {
	const cfg = `{"name":"x","mounts":[
	  {"type":"Bind","host":"/home/u/proj","guest":"/home/u/proj","options":{"readonly":false},"stat_virtualization":"strict"},
	  {"type":"Bind","host":"/data","guest":"/ro","options":{"readonly":true}},
	  {"type":"Named","name":"vol1","guest":"/vol","options":{}},
	  {"type":"Tmpfs","guest":"/tmp2","size_mib":64,"options":{}},
	  {"type":"DiskImage","host":"/d.img","guest":"/disk","format":"raw","fstype":null,"options":{}}
	]}`
	got, err := ParseStoredMounts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []Mount{
		{GuestPath: "/disk", Kind: "disk", Source: "/d.img"},
		{GuestPath: "/home/u/proj", Kind: "bind", Source: "/home/u/proj"},
		{GuestPath: "/ro", Kind: "bind", Source: "/data", Readonly: true},
		{GuestPath: "/tmp2", Kind: "tmpfs"},
		{GuestPath: "/vol", Kind: "named", Source: "vol1"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d mounts: %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("mount %d: got %+v want %+v", i, got[i], want[i])
		}
	}
	if got, _ := ParseStoredMounts(`{"name":"x"}`); len(got) != 0 {
		t.Errorf("no mounts key: %+v", got)
	}
}
