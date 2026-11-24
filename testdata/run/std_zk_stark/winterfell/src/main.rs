mod security;
use winter_air::{BatchingMethod, FieldExtension, ProofOptions};
use winter_fri::folding::apply_drp;
use winter_math::fft::{evaluate_poly, evaluate_poly_with_offset, get_twiddles, interpolate_poly};
use winter_math::fields::f64::BaseElement as B;
use winter_math::fields::CubeExtension;
use winter_math::{FieldElement, StarkField};

type E = CubeExtension<B>;

const P: u64 = 0xFFFF_FFFF_0000_0001;

fn lcg(state: &mut u64) -> u64 {
    *state = state.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);
    let mut x = *state;
    x ^= x >> 33;
    x = x.wrapping_mul(0xff51afd7ed558ccd);
    x ^= x >> 33;
    x % P
}

fn list(name: &str, values: &[u64]) {
    let text: Vec<String> = values.iter().map(|v| v.to_string()).collect();
    println!("{}: {}", name, text.join(" "));
}

fn flat(values: &[E]) -> Vec<u64> {
    values
        .iter()
        .flat_map(|v| v.to_base_elements().into_iter().map(|b| b.as_int()))
        .collect()
}

fn e(state: &mut u64) -> E {
    E::new(B::new(lcg(state)), B::new(lcg(state)), B::new(lcg(state)))
}

fn main() {
    let edges: [u64; 10] = [0, 1, 2, P - 1, P - 2, (1 << 32) - 1, 1 << 32, (1 << 32) + 1, 1 << 63, 0xFFFF_FFFE_0000_0001];
    let mut sum = vec![];
    let mut diff = vec![];
    let mut prod = vec![];
    for a in edges {
        for b in edges {
            sum.push((B::new(a) + B::new(b)).as_int());
            diff.push((B::new(a) - B::new(b)).as_int());
            prod.push((B::new(a) * B::new(b)).as_int());
        }
    }
    list("add", &sum);
    list("sub", &diff);
    list("mul", &prod);
    list("inv", &edges[1..].iter().map(|&a| B::new(a).inv().as_int()).collect::<Vec<_>>());
    list("pow7", &[0u64, 1, 2, 63, 64, 1 << 32, P - 2].iter().map(|&k| B::new(7).exp(k).as_int()).collect::<Vec<_>>());
    list("roots", &(1..=32u32).map(|k| B::get_root_of_unity(k).as_int()).collect::<Vec<_>>());

    let mut s = 1u64;
    let xs: Vec<E> = (0..6).map(|_| e(&mut s)).collect();
    list("ext_elements", &flat(&xs));
    let mut products = vec![];
    for a in &xs {
        for b in &xs {
            products.push(*a * *b);
        }
    }
    list("ext_mul", &flat(&products));
    list("ext_inv", &flat(&xs.iter().map(|a| a.inv()).collect::<Vec<_>>()));
    list("ext_frobenius", &flat(&xs.iter().map(|a| a.conjugate()).collect::<Vec<_>>()));

    let coefficients: Vec<B> = (0..32).map(|_| B::new(lcg(&mut s))).collect();
    list("poly", &coefficients.iter().map(|c| c.as_int()).collect::<Vec<_>>());
    let twiddles = get_twiddles::<B>(32);
    let mut values = coefficients.clone();
    evaluate_poly(&mut values, &twiddles);
    list("ntt", &values.iter().map(|c| c.as_int()).collect::<Vec<_>>());
    let lde = evaluate_poly_with_offset(&coefficients, &twiddles, B::GENERATOR, 4);
    list("lde4", &lde.iter().map(|c| c.as_int()).collect::<Vec<_>>());

    let ext_coefficients: Vec<E> = (0..16).map(|_| e(&mut s)).collect();
    list("ext_poly", &flat(&ext_coefficients));
    let ext_twiddles = get_twiddles::<B>(16);
    let ext_lde = evaluate_poly_with_offset(&ext_coefficients, &ext_twiddles, B::GENERATOR, 8);
    list("ext_lde8", &flat(&ext_lde));

    let evaluations: Vec<E> = (0..32).map(|_| e(&mut s)).collect();
    let alpha = e(&mut s);
    list("fold_values", &flat(&evaluations));
    list("fold_alpha", &flat(&[alpha]));
    let pairs: Vec<[E; 2]> = (0..16).map(|i| [evaluations[i], evaluations[i + 16]]).collect();
    let folded = apply_drp(&pairs, B::GENERATOR, alpha);
    list("fold_result", &flat(&folded));

    let mut inverse = values.clone();
    let inv_twiddles = winter_math::fft::get_inv_twiddles::<B>(32);
    interpolate_poly(&mut inverse, &inv_twiddles);
    assert_eq!(inverse, coefficients);

    for (queries, blowup, grinding, log_trace) in [(32usize, 8usize, 16u32, 10u32), (64, 4, 20, 12), (27, 16, 0, 8), (100, 2, 10, 14), (119, 4, 20, 20), (40, 8, 12, 5)] {
        let options = ProofOptions::new(queries, blowup, grinding, FieldExtension::Cubic, 2, 15, BatchingMethod::Linear, BatchingMethod::Linear);
        let trace = 1usize << log_trace;
        let conjectured = security::ConjecturedSecurity::compute(&options, 64, 128);
        let proven = security::ProvenSecurity::compute(&options, 64, trace, 128, 3, 3);
        println!("security {} {} {} {}: {} {} {}", queries, blowup, grinding, trace, conjectured.bits(), proven.udr_bits(), proven.ldr_bits());
    }
}
