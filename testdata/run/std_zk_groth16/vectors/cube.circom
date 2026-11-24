pragma circom 2.0.0;
template Cube() {
    signal input x;
    signal output out;
    signal sq;
    signal cu;
    sq <== x * x;
    cu <== sq * x;
    out <== cu + x + 5;
}
component main = Cube();
