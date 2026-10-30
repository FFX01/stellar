package analyzer

func newUniverse() *Scope {
	u := &Scope{Name: "universe", Kind: UniverseScope,
		symbols: map[string]Symbol{}}
	u.Declare(IntSym)
	u.Declare(FloatSym)
	u.Declare(StrSym)
	u.Declare(BoolSym)
	u.Declare(VoidSym)
	u.Declare(ErrorSym)
	u.Declare(LenSym)
	u.Declare(ResultSym)
	u.Declare(MaybeSym)
	return u
}
