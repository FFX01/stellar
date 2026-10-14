// Every form of type syntax.
type Everything object {
    a: int32
    b: Book
    c: ?Author
    d: []str
    e: Map[str, int32]
    f: Result[int32]
    g: Result
    h: Maybe[Book]
    i: fn(int32, int32) -> Result[int32]
    j: fn(str)
    k: fn(mut Book, str)
    l: []?Book
    m: Map[str, []Book]
    n: lib.Thing
    o: fn() -> fn(int32) -> str
    p: Error
}

fn takesTypes(a: mut []int32, b: ?Map[str, Book], f: fn(int32) -> int32) -> ?Book {
    return None
}
