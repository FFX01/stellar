fn divide(a: int32, b: int32) -> int32 {
    return a / b
}

fn main() {
    let big: int64 = 300
    let ok = int16(big)
    print("int16 " + str(ok))
    let zero: int32 = 0
    print(str(divide(10, zero)))
}
