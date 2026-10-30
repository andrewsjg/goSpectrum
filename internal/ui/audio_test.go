package ui

import "testing"

func TestStereoLE(t *testing.T) {
	got := StereoLE([]int16{0x1234, -1})
	want := []byte{0x34, 0x12, 0x34, 0x12, 0xFF, 0xFF, 0xFF, 0xFF}
	if len(got) != len(want) {
		t.Fatalf("len %d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d %02X", i, got[i])
		}
	}
	if len(StereoLE(nil)) != 0 {
		t.Fatal("nil")
	}
}

func TestPCMQueueSilenceAndDrop(t *testing.T) {
	var q pcmQueue
	buf := make([]byte, 4)
	n, err := q.Read(buf)
	if n != 4 || err != nil || buf[0] != 0 || buf[3] != 0 {
		t.Fatalf("empty read n %d err %v %v", n, err, buf)
	}
	q.write([]byte{1, 2, 3, 4})
	n, err = q.Read(buf)
	if n != 4 || err != nil || buf[0] != 1 || buf[3] != 4 {
		t.Fatalf("queued %v", buf)
	}

	payload := make([]byte, 48000*4+8)
	for i := range payload {
		payload[i] = byte(i)
	}
	q.write(payload)
	if len(q.buf) != 48000*4 {
		t.Fatalf("cap %d", len(q.buf))
	}
	if q.buf[len(q.buf)-1] != payload[len(payload)-1] {
		t.Fatal("dropped the newest byte")
	}
}

func TestSoundOffDoesNotOpen(t *testing.T) {
	a := OpenAudio(false)
	if a.Notice() != "sound off" || a.on {
		t.Fatalf("notice %q on %v", a.Notice(), a.on)
	}
	a.Push([]int16{1, -1})
	a.Close()
}
