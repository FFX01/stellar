// match: exhaustiveness, bindings, value and statement position.
type Shape enum {
    Circle(r: float64)
    Rect(w: float64, h: float64)
    Empty
}

type Speaker interface {
    fn hello(self) -> str
}

type Author object {
    firstName: str
}

type Robot object {
    serial: str
}

type Rock object {
    weight: int32
}

impl Speaker for Author {
    fn hello(self) -> str {
        return "Hi " + self.firstName
    }
}

impl Speaker for Robot {
    fn hello(self) -> str {
        return "Beep"
    }
}

type NotFound object {
    path: str
}

impl Error for NotFound {
    fn message(self) -> str {
        return "not found: " + self.path
    }
}

fn area(s: Shape) -> float64 {
    return match s {
        Shape.Circle(r) => 3.14 * r * r
        Shape.Rect(w, h: height) => w * height
        Shape.Empty => 0.0
    }
}

fn incomplete(s: Shape) -> float64 {
    return match s { // ERROR "match is not exhaustive: missing Shape.Rect, Shape.Empty"
        Shape.Circle(r) => r
    }
}

fn patterns(s: Shape) -> float64 {
    return match s {
        Shape.Circle(radius) => 1.0 // ERROR "Shape.Circle has no field radius"
        Shape.Circle(r) => r // ERROR "duplicate arm for Shape.Circle"
        Shape.Empty(x) => 0.0 // ERROR "Shape.Empty has no fields; remove the ()"
        Circle(r) => r // ERROR "expected a variant of Shape, written Shape.Variant"
        Shape.Square => 0.0 // ERROR "Shape has no variant Square"
        _ => 2.0
        Shape.Rect => 1.0 // ERROR "unreachable match arm"
    }
}

fn describe(s: Speaker) -> str {
    return match s {
        is Author as a => "Hi " + a.firstName
        is Robot as r => "Beep " + r.serial
        _ => s.hello()
    }
}

fn describeBad(s: Speaker) -> str {
    return match s { // ERROR "match on interface Speaker needs a _ arm"
        is Author as a => a.firstName
        is Rock => "rock" // ERROR "Rock does not implement Speaker, so this arm can never match"
        is Author => "again" // ERROR "duplicate arm for Author"
    }
}

fn find(path: str) -> Result[str] {
    if path == "" {
        return Err(NotFound{ path: path })
    }
    return Ok(path)
}

fn results(path: str) -> str {
    match find(path) {
        Ok(text) => print(text)
        Err(e) => match e {
            is NotFound as nf => print("missing: " + nf.path)
            _ => print(e.message())
        }
    }
    match find(path) { // ERROR "match is not exhaustive: missing Err"
        Ok(text) => print(text)
    }
    match find(path) {
        Ok(text) => text // ERROR "unused value of type str"
        Err(e) => print(e.message())
    }
    let n: int32 = match find(path) {
        Ok(text) => 1
        Err(e) => {
            print(e.message())
            0
        }
    }
    let m: int32 = match find(path) {
        Ok(text) => 1
        Err(e) => print(e.message()) // ERROR "print(e.message()) produces no value"
    }
    return match find(path) {
        Ok(text) => text
        Err(e) => {
            return "failed"
        }
    }
}

fn maybes(m: ?int32) -> int32 {
    return match m {
        Some(v) => v
        None => 0
    }
}

fn maybeBad(m: ?int32) -> int32 {
    return match m {
        Some(v: x) => x // ERROR "write Some(name) to bind the value"
        None(y) => 0 // ERROR "None carries no value; remove the ()"
        Ok(z) => 1 // ERROR "expected Some or None for a ?int32 subject"
    }
}

fn notMatchable(n: int32) {
    match n { // ERROR "cannot match on n (type int32)"
        _ => print("x")
    }
}

fn bindingsAreCopies(s: Speaker) -> str {
    return match s {
        is Author as a => {
            a.firstName = "x" // ERROR "a is a copy bound by a pattern"
            "x"
        }
        _ => "y"
    }
}

fn terminating(s: Shape) -> int32 {
    match s {
        Shape.Circle(r) => {
            return 1
        }
        Shape.Rect(w, h) => {
            return 2
        }
        Shape.Empty => panic("empty")
    }
}
