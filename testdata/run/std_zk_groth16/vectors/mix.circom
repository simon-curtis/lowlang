pragma circom 2.0.0;
template Mix() {
    signal input k;
    signal input a;
    signal input b;
    signal output sum;
    signal output prod;
    signal bits[8];
    var packed = 0;
    for (var i = 0; i < 8; i++) {
        bits[i] <-- (a >> i) & 1;
        bits[i] * (bits[i] - 1) === 0;
        packed += bits[i] * (1 << i);
    }
    packed === a;
    prod <== a * b;
    signal t;
    t <== prod * k;
    sum <== t + a + 3 * b + 7;
}
component main {public [k]} = Mix();
