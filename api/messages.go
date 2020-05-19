package api

import (
	"encoding/json"
	"strconv"
)

type topic string

func (t topic) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		N string `json:"name"`
	}{string(t)})
}

type subscriptionRequest struct {
	Subscribe   []topic `json:"SUBSCRIBE"`
	Unsubscribe []topic `json:"UNSUBSCRIBE"`
	SessionID   string  `json:"SESSION_ID"`
}

type SystemStat struct {
	CPU    int
	Uptime int
	Mem    int
}

func (s *SystemStat) UnmarshalJSON(data []byte) error {
	kv := make(map[string]string)
	err := json.Unmarshal(data, &kv)

	if err == nil {
		s.CPU, err = strconv.Atoi(kv["cpu"])
	}

	if err == nil {
		s.Uptime, err = strconv.Atoi(kv["uptime"])
	}

	if err == nil {
		s.Mem, err = strconv.Atoi(kv["mem"])
	}

	return err
}

type interfaceStatResp []*InterfaceStat

func (s *interfaceStatResp) UnmarshalJSON(data []byte) error {
	kv := make(map[string]struct {
		Up        string            `json:"up"`
		L1Up      string            `json:"l1up"`
		MAC       string            `json:"mac"`
		Addresses []string          `json:"addresses"`
		Stats     map[string]string `json:"stats"`
	})

	if err := json.Unmarshal(data, &kv); err != nil {
		return err
	}

	for k, v := range kv {
		st := &InterfaceStat{
			Name:      k,
			Up:        v.Up == "true",
			L1Up:      v.L1Up == "true",
			MAC:       v.MAC,
			Addresses: v.Addresses,
		}

		var err error
		st.RxPackets, err = strconv.ParseUint(v.Stats["rx_packets"], 10, 64)

		if err == nil {
			st.TxPackets, err = strconv.ParseUint(v.Stats["tx_packets"], 10, 64)
		}

		if err == nil {
			st.RxBytes, err = strconv.ParseUint(v.Stats["rx_bytes"], 10, 64)
		}

		if err == nil {
			st.TxBytes, err = strconv.ParseUint(v.Stats["tx_bytes"], 10, 64)
		}

		if err == nil {
			st.RxErrors, err = strconv.ParseUint(v.Stats["rx_errors"], 10, 64)
		}

		if err == nil {
			st.TxErrors, err = strconv.ParseUint(v.Stats["tx_errors"], 10, 64)
		}

		if err == nil {
			st.RxDropped, err = strconv.ParseUint(v.Stats["rx_dropped"], 10, 64)
		}

		if err == nil {
			st.TxDropped, err = strconv.ParseUint(v.Stats["tx_dropped"], 10, 64)
		}

		if err == nil {
			st.RxBytesPerSec, err = strconv.ParseUint(v.Stats["rx_bps"], 10, 64)
		}

		if err == nil {
			st.TxBytesPerSec, err = strconv.ParseUint(v.Stats["tx_bps"], 10, 64)
		}

		if err == nil {
			st.Multicast, err = strconv.ParseUint(v.Stats["multicast"], 10, 64)
		}

		*s = append(*s, st)
	}

	return nil
}

type InterfaceStat struct {
	Name      string
	Up        bool
	L1Up      bool
	MAC       string
	Addresses []string

	RxPackets     uint64
	TxPackets     uint64
	RxBytes       uint64
	TxBytes       uint64
	RxErrors      uint64
	TxErrors      uint64
	RxDropped     uint64
	TxDropped     uint64
	RxBytesPerSec uint64
	TxBytesPerSec uint64
	Multicast     uint64
}
