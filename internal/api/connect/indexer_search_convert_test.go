package apiconnect

import (
	"purser/internal/ports"
	"testing"

	acquisitionv1 "purser/gen/go/purser/acquisition/v1"
)

func TestProtocolToProto(t *testing.T) {
	tests := []struct {
		in   ports.Protocol
		want acquisitionv1.Protocol
	}{
		{ports.ProtocolTorrent, acquisitionv1.Protocol_PROTOCOL_TORRENT},
		{ports.ProtocolUsenet, acquisitionv1.Protocol_PROTOCOL_USENET},
		{ports.Protocol("unknown"), acquisitionv1.Protocol_PROTOCOL_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := protocolToProto(tt.in); got != tt.want {
			t.Errorf("protocolToProto(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
