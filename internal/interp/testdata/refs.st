// Ref[T]: one shared, writable target reached from many places.
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

fn touch(a: Ref[Author], n: str) {
    a.name = n
}

fn rename(a: mut Author, n: str) {
    a.name = n
}

fn main() {
    let herbert: Ref[Author] = Ref(Author{ name: "Herbert" })
    let dune = Book{ title: "Dune", author: herbert }
    let messiah = Book{ title: "Dune Messiah", author: herbert }

    herbert.name = "F. Herbert"
    print(dune.author.name + " / " + messiah.author.name)

    touch(herbert, "Frank Herbert")
    print(dune.author.greet())

    mut messiah.author.rename("FH")
    print(herbert.name)

    rename(mut herbert.value, "Frank")
    print(dune.author.name)

    let snapshot = herbert.value
    herbert.value = Author{ name: "replaced" }
    print("snapshot " + snapshot.name + ", now " + dune.author.name)

    let copyOfBook = dune
    copyOfBook.author.name = "shared through a copied book"
    print(messiah.author.name)

    let counter: Ref[int32] = Ref(0)
    let inc = fn() {
        counter.value = counter.value + 1
    }
    inc()
    inc()
    inc()
    print("counter " + str(counter.value))

    let other: Ref[Author] = Ref(Author{ name: "shared through a copied book" })
    print("same target: " + str(dune.author == messiah.author) + ", equal contents but different Ref: " + str(dune.author == other))
}
