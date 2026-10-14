type NotFound object {
    key: str
}

impl Error for NotFound {
    fn message(self) -> str {
        return "not found: " + self.key
    }
}

fn find(key: str) -> Result[int32] {
    return Err(NotFound{ key: key })
}

fn main() -> Result {
    let v = try find("x") else |e| {
        print("caught: " + e.message())
        return Err(wrap(e, "looking up x"))
    }
    return Ok()
}
