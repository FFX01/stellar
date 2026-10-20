pub fn add(a: int, b: int) -> Result[int] {
    var sum: int = a + b;
    return Ok(sum);
}

pub fn handle[T](arg: T) -> Action {
    return Action(item=arg);
}
