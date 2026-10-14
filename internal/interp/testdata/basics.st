// Arithmetic, strings, conversions, control flow, functions and closures.
fn fib(n: int32) -> int32 {
    if n < 2 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

fn apply(f: fn(int32) -> int32, x: int32) -> int32 {
    return f(x)
}

fn makeAdder(k: int32) -> fn(int32) -> int32 {
    return fn(x: int32) -> int32 {
        return x + k
    }
}

fn classify(n: int32) -> str {
    return if n < 0 { "negative" } else if n == 0 { "zero" } else { "positive" }
}

fn firstOver(xs: []int32, limit: int32) -> ?int32 {
    for x in xs {
        if x > limit {
            return Some(x)
        }
    }
    return None
}

fn main() {
    let a: int32 = 7
    let b: int32 = 3
    print(str(a + b) + " " + str(a - b) + " " + str(a * b) + " " + str(a / b) + " " + str(a % b))
    let x: float64 = 7.0
    print(str(x / 2.0) + " " + str(x * 2.0))
    let small: int8 = -128
    print(str(small) + " " + str(int64(small) * 2))
    let f: float64 = 3.99
    print(str(int32(f)) + " " + str(float64(a)) + " " + str(true))
    print("fib(20) = " + str(fib(20)))
    print(classify(-5) + " " + classify(0) + " " + classify(5))

    var total: int64 = 0
    let limit: int64 = 10
    for i in 0..limit {
        if i == 3 {
            continue
        }
        if i == 8 {
            break
        }
        total = total + i
    }
    print("total " + str(total))

    var n: int32 = 0
    while n < 5 {
        n = n + 1
    }
    print("n " + str(n))

    let addTen = makeAdder(10)
    print("apply " + str(apply(addTen, 5)))
    let double = fn(v: int32) -> int32 {
        return v * 2
    }
    print("double " + str(apply(double, 21)))

    let xs: []int32 = [4, 9, 2, 15]
    let found: int32 = try firstOver(xs, 8) else 0
    let missing: int32 = try firstOver(xs, 100) else -1
    print("found " + str(found) + ", missing " + str(missing))
    let third: int32 = try xs[2]
    let tenth: int32 = try xs[10] else -1
    print("third " + str(third) + ", tenth " + str(tenth) + ", len " + str(len(xs)))

    let ages: Map[str, int32] = ["ann": 31, "bob": 42]
    let ann: int32 = try ages["ann"]
    let cy: int32 = try ages["cy"] else 0
    print("ann " + str(ann) + ", cy " + str(cy))
    for name in ages {
        print("key " + name)
    }

    let label: str = match firstOver(xs, 8) {
        Some(v) => "some " + str(v)
        None => "none"
    }
    print(label)
    let same: bool = [1, 2] == xs
    print("equal arrays: " + str(same) + " " + str(xs == [4, 9, 2, 15]))
}
