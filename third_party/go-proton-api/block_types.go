package proton

// Block is a block of file contents. They are split in 4MB blocks although this number may change in the future.
// Each block is its own data packet separated from the key packet which is held by the node,
// which means the sessionKey is the same for every block.
type Block struct {
	Index int

	BareURL string // URL to the block
	Token   string // Token for download URL

	Hash           string // Encrypted block's sha256 hash, in base64
	EncSignature   string // Encrypted signature of the block
	SignatureEmail string // Email used to sign the block
}

type BlockUploadReq struct {
	AddressID  string
	ShareID    string
	LinkID     string
	RevisionID string

	BlockList []BlockUploadInfo
}

// BlockUploadVerifier carries the per-block verification token required by
// the current upload protocol (XOR of the verification code with the first
// 32 bytes of the encrypted block, base64-encoded).
type BlockUploadVerifier struct {
	Token string
}

type BlockUploadInfo struct {
	Index        int
	Size         int64
	EncSignature string
	Hash         string

	Verifier *BlockUploadVerifier
}

// RevisionVerification is returned by the v2 verification endpoint: a
// 32-byte verification code used to derive per-block upload tokens, and the
// content key packet.
type RevisionVerification struct {
	VerificationCode string
	ContentKeyPacket string
}

type BlockUploadLink struct {
	Token   string
	BareURL string
}
