// Ref[T]: shared, always-writable references (design §5.5).
type Author object {
    name: str
}

impl Author {
    pub fn rename(mut self, n: str) {
        self.name = n
    }
    pub fn greet(self) -> str {
        return "Hi " + self.name
    }
}

type Book object {
    title: str
    author: Ref[Author]
}

fn touch(a: Ref[Author]) {
    a.name = "changed"
}

fn refs() {
    let herbert: Ref[Author] = Ref(Author{ name: "Herbert" })
    let inferred = Ref(Author{ name: "x" })
    let counter: Ref[int32] = Ref(0)
    let bad = Ref(0) // ERROR "cannot infer the type of integer literal 0"

    herbert.name = "F. Herbert"
    herbert.value = Author{ name: "Frank" }
    counter.value = counter.value + 1
    let n: int32 = counter.value
    let g: str = herbert.greet()
    mut herbert.rename("Frank Herbert")
    herbert.rename("x") // ERROR "rename changes herbert; mark the call"
    mut herbert.value.rename("y")
    touch(herbert)

    let dune = Book{ title: "Dune", author: herbert }
    dune.author.name = "via let book"
    dune.title = "x" // ERROR "dune is declared with let"
    mut dune.author.rename("z")

    herbert = inferred // ERROR "herbert is declared with let"
    var current = herbert
    current = inferred

    let same: bool = herbert == inferred
    let mixed: Ref[int32] = herbert // ERROR "cannot use herbert (type Ref[Author]) as Ref[int32]"
    let missing = herbert.age // ERROR "Author has no field or method age"

    let inc = fn() {
        counter.value = counter.value + 1
    }
    var notShared: int32 = 0
    let bump = fn() {
        notShared = 1 // ERROR "a closure cannot capture var notShared"
    }
}
