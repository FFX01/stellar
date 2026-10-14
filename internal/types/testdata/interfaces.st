// Interface values, Self, impls for primitives and array covariance.
type Describable interface {
    fn describe(self) -> str
    fn twin(self) -> Self
}

type Item object {
    name: str
}

impl Describable for Item {
    fn describe(self) -> str {
        return self.name
    }
    fn twin(self) -> Item {
        return Item{ name: self.name }
    }
}

impl Describable for int32 {
    fn describe(self) -> str {
        return str(self)
    }
    fn twin(self) -> int32 {
        return self
    }
}

type Counter interface {
    fn bump(mut self)
}

type Tally object {
    n: int32
}

impl Counter for Tally {
    fn bump(mut self) {
        self.n = self.n + 1
    }
}

fn describeAll(items: []Describable) -> str {
    var out: str = ""
    for d in items {
        out = out + d.describe()
        let t: Describable = d.twin()
    }
    return out
}

fn use(n: int32, it: Item, c: Counter, mc: mut Counter, t: Tally) {
    let s: str = n.describe()
    let d: Describable = n
    let d2: Describable = it
    let items: []Item = [it]
    let all: str = describeAll(items)
    let mixed: []Describable = [n, it]
    let tw: Item = it.twin()
    mut c.bump() // ERROR "parameter c is not mut"
    mut mc.bump()
    mc.bump() // ERROR "bump changes mc; mark the call: mut mc.bump(...)"
    var asCounter: Counter = t
    mut asCounter.bump()
    let bad: Describable = c // ERROR "cannot use c (type Counter) as Describable"
    let f: fn(Describable) -> str = Describable.describe
}
