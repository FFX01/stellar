// Result, Maybe and every form of try.
pub fn save(b: Book) -> Result {
    return Ok()
}

pub fn findByIsbn(books: []Book, isbn: str) -> ?Book {
    for b in books {
        if b.isbn == isbn {
            return Some(b)
        }
    }
    return None
}

fn tries(books: []Book, b: Book) -> Result[int32] {
    let sum = try add(1, 2)
    let sum2 = try add(1, 2) else 0
    let sum3 = try add(1, 2) else |e| fallback(e)
    let sum4 = try add(1, 2) else |e| {
        return Err(wrap(e, "adding"))
    }
    let first = try books[0] else {
        return Err("no books")
    }
    let total: int32 = try add(sum, sum2) + 1
    try save(b)
    let _ = try add(3, 4)
    let _ = findByIsbn(books, "x")
    return Ok(sum + sum2 + sum3 + sum4)
}

fn handle() {
    match add(1, 2) {
        Ok(v) => print(str(v))
        Err(e) => print(e.message())
    }
    match findByIsbn(books, "x") {
        Some(b) => print(b.title)
        None => print("missing")
    }
}
