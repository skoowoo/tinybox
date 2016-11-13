package tinybox

func init() {
	registerSetter(&defaultMem{})
}

type defaultMem struct{}

func (d defaultMem) IsSubsys(typ string) bool {
	return typ == subsysMEM
}

func (d defaultMem) Validate(opt *CGroupOptions) error {
	return nil
}

func (d defaultMem) Write(opt *CGroupOptions, dir string) error {
	return nil
}
