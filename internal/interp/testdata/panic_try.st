fn parse(s: str) -> Result[int32] {
    if s == "" {
        return Err("empty input")
    }
    return Ok(1)
}

fn main() {
    let a = try parse("x")
    print("a " + str(a))
    let b = try parse("")
    print("unreachable")
}
