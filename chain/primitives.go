package chain

type Block struct {
	Hash          string `json:"hash"`
	Confirmations int    `json:"confirmations"`
	Height        int    `json:"height"`
	Version       int    `json:"version"`
	MerkleRoot    string `json:"merkleroot"`
	Time          int64  `json:"time"`
	Tx            []Tx   `json:"tx"`
	//implement other fields iff nescairy
}

type Tx struct {
	TxID     string `json:"txid"`
	Hash     string `json:"hash"`
	Version  int    `json:"version"`
	Size     int    `json:"size"`
	Vin      []Vin  `json:"vin"`
	Vout     []Vout `json:"vout"`
	//implement other fields iff nescairy
}

type Vin struct {
	TxID        string      `json:"txid,omitempty"`        // Previous transaction ID
	Vout        int         `json:"vout,omitempty"`        // Output index from previous tx
	ScriptSig   ScriptSig   `json:"scriptSig,omitempty"`   // Unlocking script
	Sequence    uint32      `json:"sequence"`              // Sequence number
	TxInWitness []string    `json:"txinwitness,omitempty"` // Witness data (SegWit)
	Coinbase    string      `json:"coinbase,omitempty"`    // Coinbase input (if any)
}

type ScriptSig struct {
	Asm string `json:"asm"` // Human-readable script
	Hex string `json:"hex"` // Hex-encoded script
}

type Vout struct {
	Value        float64      `json:"value"`
	N            int          `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}


type ScriptPubKey struct {
	Asm       string   `json:"asm"`       // Assembly format of the script
	Hex       string   `json:"hex"`       // Hex encoding of the script
	ReqSigs   int      `json:"reqSigs,omitempty"`   // Number of required signatures
	Type      string   `json:"type,omitempty"`      // Script type (e.g., "pubkeyhash")
	Addresses []string `json:"addresses,omitempty"` // Associated Bitcoin addresses
}
