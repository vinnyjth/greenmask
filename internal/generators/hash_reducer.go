package generators

type HashReducer struct {
	g    Generator
	size int
}

func NewHashReducer(g Generator, size int) *HashReducer {
	return &HashReducer{
		g:    g,
		size: size,
	}
}

func (hr *HashReducer) Generate(data []byte) (res []byte, err error) {
	res, err = hr.g.Generate(res)
	if err != nil {
		return nil, err
	}

	return res[:hr.size], nil
}

func (hr *HashReducer) Size() int {
	return hr.size
}
