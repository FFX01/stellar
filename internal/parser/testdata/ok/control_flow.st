// if/match as expressions, block values, loops and type tests.
fn area(s: Shape) -> float64 {
    return match s {
        Shape.Circle(r) => 3.14 * r * r
        Shape.Rect(w, h: height) => w * height
        Shape.Empty => 0.0
    }
}

fn price(item: Item) -> float64 {
    let p: float64 = match item {
        is Book as b => b.basePrice
        is Magazine as m => {
            let base: float64 = m.issuePrice
            let discount: float64 = base * 0.1
            base - discount
        }
        is Pamphlet => 0.0
        _ => 1.0
    }
    return p
}

fn describe(s: Speaker) -> str {
    if s is Author as a {
        return "Hi " + a.firstName
    } else if s is Robot {
        return "Beep"
    } else {
        return s.hello()
    }
}

fn loops(n: int32) {
    let label: str = if n > 0 { "pos" } else { "neg" }
    var i: int32 = 0
    while i < n and not done(i) {
        i = i + 1
        if i == 5 {
            continue
        }
        if i > 10 {
            break
        }
    }
    for x in Range{ lo: 0, hi: n } {
        let _ = x
    }
    if origin == Point{ x: 0, y: 0 } {
        let _ = origin
    }
    for x in (Range{ lo: 0, hi: n }) {
        let _ = x
    }
    if c {
        let _ = compute()
    }
}
