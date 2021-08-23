package adr

import "github.com/brocaar/chirpstack-network-server/v3/adr"

// DefaultHandler implements the default ADR handler.
type LabSCimHandler struct{}

// ID returns the default ID.
func (h *LabSCimHandler) ID() (string, error) {
	return "labscimadr", nil
}

// Name returns the default name.
func (h *LabSCimHandler) Name() (string, error) {
	return "LabSCim ADR algorithm", nil
}

// Handle handles the ADR request.
func (h *LabSCimHandler) Handle(req adr.HandleRequest) (adr.HandleResponse, error) {
	// This defines the default response, which is equal to the current device
	// state.
	resp := adr.HandleResponse{
		DR:           req.DR,
		TxPowerIndex: req.TxPowerIndex,
		NbTrans:      req.NbTrans,
	}

	// If ADR is disabled, return with current values.
	if !req.ADR {
		return resp, nil
	}

	// Lower the DR only if it exceeds the max. allowed DR.
	if req.DR > req.MaxDR {
		resp.DR = req.MaxDR
	}

	// Set the new NbTrans.
	resp.NbTrans = h.getNbTrans(req.NbTrans, h.getPacketLossPercentage(req))

	if h.getHistoryCount(req) > h.requiredHistoryCount() {

		// Calculate the number of 'steps'.
		snrM := h.getMeanSNR(req)
		snrMargin := snrM - req.RequiredSNRForDR - req.InstallationMargin
		nStep := int(snrMargin / 3)

		resp.TxPowerIndex, resp.DR = h.getIdealTxPowerIndexAndDR(nStep, resp.TxPowerIndex, resp.DR, req.MaxTxPowerIndex, req.MaxDR)
	}

	return resp, nil
}

func (h *LabSCimHandler) pktLossRateTable() [][3]int {
	return [][3]int{
		{1, 1, 2},
		{1, 2, 3},
		{2, 3, 3},
		{3, 3, 3},
	}
}

func (h *LabSCimHandler) getMaxSNR(req adr.HandleRequest) float32 {
	var snrM float32 = -999
	for _, m := range req.UplinkHistory {
		if m.MaxSNR > snrM {
			snrM = m.MaxSNR
		}
	}
	return snrM
}

func (h *LabSCimHandler) getMeanSNR(req adr.HandleRequest) float32 {
	var sumSNR float32 = 0
	var countSNR float32 = 0
	for _, m := range req.UplinkHistory {
		if req.TxPowerIndex == m.TXPowerIndex {
			sumSNR = sumSNR + m.MaxSNR
			countSNR = countSNR + 1
		}
	}
	return sumSNR / countSNR
}

// getHistoryCount returns the history count with equal TxPowerIndex.
func (h *LabSCimHandler) getHistoryCount(req adr.HandleRequest) int {
	var count int
	for _, uh := range req.UplinkHistory {
		if req.TxPowerIndex == uh.TXPowerIndex {
			count++
		}
	}
	return count
}

func (h *LabSCimHandler) requiredHistoryCount() int {
	return 10
}

func (h *LabSCimHandler) getIdealTxPowerIndexAndDR(nStep, txPowerIndex, dr, maxTxPowerIndex, maxDR int) (int, int) {
	if nStep == 0 {
		return txPowerIndex, dr
	}

	if nStep > 0 {
		if dr < maxDR {
			// Increase the DR.
			dr++
		} else if txPowerIndex < maxTxPowerIndex {
			// Decrease the TxPower.
			txPowerIndex++
		}
		nStep--
	} else {
		if txPowerIndex > 0 {
			// Increase TxPower.
			txPowerIndex--
		}
		nStep++
	}

	return h.getIdealTxPowerIndexAndDR(nStep, txPowerIndex, dr, maxTxPowerIndex, maxDR)
}

func (h *LabSCimHandler) getNbTrans(currentNbTrans int, pktLossRate float32) int {
	if currentNbTrans < 1 {
		currentNbTrans = 1
	}

	if currentNbTrans > 3 {
		currentNbTrans = 3
	}

	if pktLossRate < 5 {
		return h.pktLossRateTable()[0][currentNbTrans-1]
	} else if pktLossRate < 10 {
		return h.pktLossRateTable()[1][currentNbTrans-1]
	} else if pktLossRate < 30 {
		return h.pktLossRateTable()[2][currentNbTrans-1]
	}

	return h.pktLossRateTable()[3][currentNbTrans-1]
}

func (h *LabSCimHandler) getPacketLossPercentage(req adr.HandleRequest) float32 {
	if len(req.UplinkHistory) < h.requiredHistoryCount() {
		return 0
	}

	var lostPackets uint32
	var previousFCnt uint32

	for i, m := range req.UplinkHistory {
		if i == 0 {
			previousFCnt = m.FCnt
			continue
		}

		lostPackets += m.FCnt - previousFCnt - 1 // there is always an expected difference of 1
		previousFCnt = m.FCnt
	}

	return float32(lostPackets) / float32(len(req.UplinkHistory)) * 100
}
