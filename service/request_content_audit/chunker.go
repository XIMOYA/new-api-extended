// service/request_content_audit/chunker.go
// 内容定义分块：用固定 gear 哈希表切分明文，保证同一段内容在任何进程、任何时间都切在相同位置，
// 相邻请求重复的会话前缀才能命中同一批内容寻址块。
package request_content_audit

const (
	contentChunkMinSize = 1 << 10
	contentChunkMaxSize = 16 << 10
	// 掩码低 12 位为 0 时切块，平均块长约 4KB。
	contentChunkMask = uint64(1<<12) - 1
)

var contentChunkGear = buildContentChunkGear()

func buildContentChunkGear() [256]uint64 {
	var table [256]uint64
	state := uint64(0x9E3779B97F4A7C15)
	for index := range table {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		table[index] = state
	}
	return table
}

// splitContentChunks 把明文切成内容定义块，返回的切片共享底层数组，调用方不要修改。
func splitContentChunks(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	chunks := make([][]byte, 0, len(data)/contentChunkMinSize+1)
	for len(data) > 0 {
		size := nextContentChunkSize(data)
		chunks = append(chunks, data[:size])
		data = data[size:]
	}
	return chunks
}

func nextContentChunkSize(data []byte) int {
	if len(data) <= contentChunkMinSize {
		return len(data)
	}
	limit := len(data)
	if limit > contentChunkMaxSize {
		limit = contentChunkMaxSize
	}
	var hash uint64
	for index := contentChunkMinSize; index < limit; index++ {
		hash = (hash << 1) + contentChunkGear[data[index]]
		if hash&contentChunkMask == 0 {
			return index + 1
		}
	}
	return limit
}
