package bitpack

func Pack(vals []int64) byte {
	//int[] inputValues = new int[] {0, 1, 2, 3, 4, 5, 6, 7};

	//assertThat(encodedBytesRepresentation).isEqualTo("10001000 11000110 11111010");
	return (byte(vals[0]&1) |
		byte((vals[1]&1)<<1) |
		byte((vals[2]&1)<<2) |
		byte((vals[3]&1)<<3) |
		byte((vals[4]&1)<<4) |
		byte((vals[5]&1)<<5) |
		byte((vals[6]&1)<<6) |
		byte((vals[7]&1)<<7)) & 255
}

func Unack(b byte) []int64 {

}
