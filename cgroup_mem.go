package tinybox

func init() {
	registerSetter(&defaultMemSet{})
}

type defaultMemSet struct {
}

func (s defaultMemSet) IsSubsys(typ string) bool {
	return typ == subsysMEM
}

func (s defaultMemSet) Validate(opt *CGroupOptions) error {
	return nil
}

func (s defaultMemSet) Write(opt *CGroupOptions, dir string) error {
	return nil
}
