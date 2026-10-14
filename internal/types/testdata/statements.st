// let/var, assignment, loops and discard rules.
fn statements(xs: []int32, n: int64) {
    let a: int32 = 1
    a = 2 // ERROR "cannot assign to a: a is declared with let; use var"
    var b: int32 = 1
    b = b + 1
    b = 1.5 // ERROR "cannot use float literal"
    n = 3 // ERROR "parameter n is not mut"
    undefinedName = 1 // ERROR "undefined: undefinedName"

    var i: int64 = 0
    while i < n {
        i = i + 1
        if i == 5 {
            continue
        }
        if i > 10 {
            break
        }
    }
    for k in 0..n {
        let _ = k
    }
    for x in xs {
        let y: int32 = x
    }
    for q in 0..10 { // ERROR "cannot infer the type of integer literal 0"
    }
    for r in a { // ERROR "cannot iterate over a (type int32)"
    }
    break // ERROR "break is only allowed inside a loop"

    a + 1 // ERROR "unused value of type int32: use let _ = ... to discard it"
    let _ = a + 1
    let x: int32 = 1
    let x: int32 = 2 // ERROR "x is already declared in this scope"
    let str: int32 = 1 // ERROR "cannot use built-in name str as a variable name"
    let first = try xs[0]
    let c: int32 = first
    let d: int32 = xs[1] // ERROR "use try to unwrap it"
    let e: int32 = try xs[a] // ERROR "cannot use a (type int32) as int64"
}

fn noResult() {
    return 1 // ERROR "function has no result type, so return takes no value"
}

fn missing(a: int32) -> int32 { // ERROR "missing return at end of function missing"
    if a > 0 {
        return a
    }
}

fn allPaths(a: int32) -> int32 {
    if a > 0 {
        return a
    } else {
        return 0
    }
}

fn bareReturn() -> int32 {
    return // ERROR "missing return value of type int32"
}
