fn main() {
    for idx, i in range(0, 10) {
        print(i);
    }
    var a: []int = [1, 2, 3];
    for idx, i in a {
        print(i);
    }
    for a > b or false {
        a = a - 1;
    }
    var a: []int = for _, i in range(0, 10) { >> i; };
    for {
        print(a);
        a += 1;
        if a > 10 {
            break;
        }
    }
}
