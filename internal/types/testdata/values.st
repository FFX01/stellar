// Object, array and map literals; mutability through fields and indexes.
type Author object {
    firstName: str
    lastName: str
}

type Book object {
    title: str
    author: Author
    tags: []str
}

type Speaker interface {
    fn hello(self) -> str
}

impl Speaker for Author {
    fn hello(self) -> str {
        return "Hi " + self.firstName
    }
}

type Robot object {
    serial: str
}

fn literals(a: Author) {
    let b = Book{ title: "Dune", author: a, tags: ["sf"] }
    let m = Author{ firstName: "F", lastName: "H" }
    let c = Book{ title: "x", author: a } // ERROR "missing field tags in Book literal"
    let d = Book{ title: "x", author: a, tags: [], pages: 1 } // ERROR "Book has no field pages"
    let e = Book{ title: "x", title: "y", author: a, tags: [] } // ERROR "duplicate field title"
    let f = Unknown{ x: "1" } // ERROR "undefined type Unknown"
    let g = Book{ title: "x", author: m, tags: [] }

    let xs: []int32 = [1, 2, 3]
    let none: []int32 = []
    let ys = [1, 2] // ERROR "cannot infer the type of array literal"
    let ages: Map[str, int32] = ["ann": 31, "bob": 42]
    let empty: Map[str, int32] = [:]
    let bad: Map[str, int32] = ["ann": "x"] // ERROR "cannot use string literal \"x\" as int32"
    let wrong: []int32 = ["a": 1] // ERROR "cannot use map literal as []int32"
}

fn typeTests(s: Speaker) -> str {
    if s is Author as au and au.firstName == "F" {
        return au.lastName
    }
    if s is Robot { // ERROR "Robot does not implement Speaker"
        return "robot"
    }
    let ok: bool = s is Author as named // ERROR "as bindings are only allowed in if and while conditions"
    let a = Author{ firstName: "a", lastName: "b" }
    if a is Author { // ERROR "type tests need an interface value"
        return ""
    }
    return au.firstName // ERROR "undefined: au"
}
