// if as an expression, block values and closures (design §5.4, §8.4).
fn blocks(c: bool, n: int32) -> int32 {
    let x: int32 = if c { 1 } else { 2 }
    let y = if c { 1 } else { 2 } // ERROR "cannot infer the type of integer literal 1"
    let z: int32 = if c { 1 } // ERROR "if used as a value needs an else branch"
    let w: int32 = if c {
        let t: int32 = n * 2
        t + 1
    } else {
        return 0
    }
    let v: int32 = if c { n } else { let q: int32 = 1 } // ERROR "block produces no value"
    let u = if c { n } else { true } // ERROR "cannot use true (type bool) as int32"
    let s: str = if c { "a" } else if n > 0 { "b" } else { "c" }
    if c {
        n // ERROR "unused value of type int32"
    }
    return x + w
}

fn closures(p: mut int32, limit: int32) {
    var count: int32 = 0
    let total: int32 = 0
    let inc = fn() {
        count = count + 1 // ERROR "a closure cannot capture var count; copy it into a let first, or share it with a Ref"
    }
    let readVar = fn() -> int32 {
        return count // ERROR "a closure cannot capture var count"
    }
    let snapshot = count
    let show = fn() -> int32 {
        return snapshot + total + limit
    }
    let assignLet = fn() {
        total = 1 // ERROR "total is declared with let"
    }
    let bad = fn() {
        p = 2 // ERROR "a closure cannot capture mut parameter p"
    }
    let nested = fn() -> fn() -> int32 {
        let local: int32 = 1
        return fn() -> int32 {
            return local + total
        }
    }
    let f: fn(int32) -> int32 = fn(a: int32) -> int32 {
        return a * 2
    }
    let h: fn(mut int32) = fn(x: mut int32) {
        x = x + 1
    }
    var inner: int32 = 0
    let ownVar = fn() {
        var mine: int32 = 0
        mine = mine + 1
    }
    while true {
        let k = fn() {
            break // ERROR "break is only allowed inside a loop"
        }
        break
    }
}
