// Enums, match, interfaces, dynamic dispatch, type tests and Error values.
type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
}

type Describable interface {
    fn describe(self) -> str
}

type Item object {
    name: str
}

impl Describable for Item {
    fn describe(self) -> str {
        return "item " + self.name
    }
}

impl Describable for int32 {
    fn describe(self) -> str {
        return "number " + str(self)
    }
}

impl Describable for Shape {
    fn describe(self) -> str {
        return match self {
            Shape.Circle(r) => "circle r=" + str(r)
            Shape.Rect(w, h: height) => "rect " + str(w) + "x" + str(height)
            Shape.Empty => "empty"
        }
    }
}

type Counter interface {
    fn bump(mut self)
    fn count(self) -> int32
}

type Tally object {
    n: int32
}

impl Counter for Tally {
    fn bump(mut self) {
        self.n = self.n + 1
    }
    fn count(self) -> int32 {
        return self.n
    }
}

type NotFound object {
    key: str
}

impl Error for NotFound {
    fn message(self) -> str {
        return "not found: " + self.key
    }
}

fn area(s: Shape) -> float64 {
    return match s {
        Shape.Circle(r) => 3.0 * r * r
        Shape.Rect(w, h) => w * h
        Shape.Empty => 0.0
    }
}

fn lookup(key: str) -> Result[int32] {
    if key == "missing" {
        return Err(NotFound{ key: key })
    }
    if key == "" {
        return Err("empty key")
    }
    return Ok(42)
}

fn explain(r: Result[int32]) -> str {
    return match r {
        Ok(v) => "ok " + str(v)
        Err(e) => match e {
            is NotFound as nf => "missing key " + nf.key
            _ => "other error: " + e.message()
        }
    }
}

fn main() {
    let shapes: []Shape = [Shape.Circle(r: 1.0), Shape.Rect(w: 2.0, h: 3.0), Shape.Empty]
    for s in shapes {
        print(s.describe() + " area " + str(area(s)))
    }

    let n: int32 = 7
    let things: []Describable = [Item{ name: "lamp" }, n, Shape.Empty]
    for t in things {
        print(t.describe())
        if t is Item as it {
            print("  an item named " + it.name)
        }
    }
    let describeFn: fn(Describable) -> str = Describable.describe
    print(describeFn(Item{ name: "via function value" }))
    let bound = Item{ name: "bound" }.describe
    print(bound())

    var c: Counter = Tally{ n: 0 }
    mut c.bump()
    mut c.bump()
    print("count " + str(c.count()))

    print(explain(lookup("a")))
    print(explain(lookup("missing")))
    print(explain(lookup("")))

    let wrapped = wrap(wrap(NotFound{ key: "k" }, "inner"), "outer")
    print(wrapped.message())
    match cause(wrapped) {
        Some(inner) => {
            print("cause: " + inner.message())
            match cause(inner) {
                Some(root) => match root {
                    is NotFound as nf => print("root is NotFound(" + nf.key + ")")
                    _ => print("root is something else")
                }
                None => print("no root")
            }
        }
        None => print("no cause")
    }
    print("same enum: " + str(Shape.Rect(w: 2.0, h: 3.0) == Shape.Rect(w: 2.0, h: 3.0)))
}
