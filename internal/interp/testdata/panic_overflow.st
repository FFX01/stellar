fn grow(n: int8) -> int8 {
    return n * 2
}

fn main() {
    var n: int8 = 1
    while true {
        n = grow(n)
        print(str(n))
    }
}
