use blake2b_simd::Params as Blake2bParams;
use ff::{Field, FromUniformBytes, PrimeField, WithSmallOrderMulGroup};
use group::{prime::PrimeCurveAffine, Curve, Group, GroupEncoding};
use halo2_proofs::arithmetic::{eval_polynomial, CurveAffine, CurveExt};
use halo2_proofs::pasta::{EpAffine, EqAffine, Fp, Fq};
use halo2_proofs::poly::commitment::{create_proof, verify_proof, Blind, Params};
use halo2_proofs::poly::EvaluationDomain;
use halo2_proofs::transcript::{
    Blake2bRead, Blake2bWrite, Challenge255, Transcript, TranscriptRead, TranscriptWrite,
};
use rand_core::OsRng;

use crate::util::*;

fn hash64(msg: &[u8]) -> [u8; 64] {
    *blake2b_simd::blake2b(msg).as_array()
}

/// The 12 field elements: 0, 1, 2, p-1, p-2, (p-1)/2, (p+1)/2, 2^64, 2^128+7,
/// 2^254+12345, from_uniform_bytes(blake2b-512("a")), from_uniform_bytes(blake2b-512("b")).
pub fn elems<F: PrimeField + FromUniformBytes<64>>() -> Vec<F> {
    let two = F::from(2);
    let two64 = F::from(u64::MAX) + F::ONE;
    vec![
        F::ZERO,
        F::ONE,
        two,
        -F::ONE,
        -two,
        -F::TWO_INV,
        F::TWO_INV,
        two64,
        two64.square() + F::from(7),
        two.pow_vartime([254u64]) + F::from(12345),
        F::from_uniform_bytes(&hash64(b"a")),
        F::from_uniform_bytes(&hash64(b"b")),
    ]
}

fn uniform_inputs() -> Vec<[u8; 64]> {
    let mut up = [0u8; 64];
    let mut down = [0u8; 64];
    for i in 0..64 {
        up[i] = i as u8;
        down[i] = (63 - i) as u8;
    }
    let mut low_ff = [0u8; 64];
    low_ff[..32].fill(0xff);
    vec![[0u8; 64], [0xffu8; 64], up, down, hash64(b"x"), low_ff]
}

pub fn field_records<F>(p: &str, out: &mut Out)
where
    F: PrimeField + FromUniformBytes<64> + WithSmallOrderMulGroup<3>,
{
    out.rec(&format!("{p}_modulus"), vec![hex(&modulus_le::<F>())]);
    out.rec(&format!("{p}_root_of_unity"), vec![fe(&F::ROOT_OF_UNITY)]);
    out.rec(&format!("{p}_delta"), vec![fe(&F::DELTA)]);
    out.rec(&format!("{p}_zeta"), vec![fe(&F::ZETA)]);
    out.rec(&format!("{p}_two_inv"), vec![fe(&F::TWO_INV)]);

    let e = elems::<F>();
    out.rec(&format!("{p}_elems"), e.iter().map(fe).collect());

    let first8 = &e[..8];
    let mut add = vec![];
    let mut sub = vec![];
    let mut mul = vec![];
    for x in first8 {
        for y in first8 {
            add.push(fe(&(*x + *y)));
            sub.push(fe(&(*x - *y)));
            mul.push(fe(&(*x * *y)));
        }
    }
    out.rec(&format!("{p}_add"), add);
    out.rec(&format!("{p}_sub"), sub);
    out.rec(&format!("{p}_mul"), mul);
    out.rec(&format!("{p}_neg"), e.iter().map(|x| fe(&-*x)).collect());
    out.rec(&format!("{p}_square"), e.iter().map(|x| fe(&x.square())).collect());
    out.rec(
        &format!("{p}_invert"),
        e.iter()
            .map(|x| Option::<F>::from(x.invert()).map_or("none".to_string(), |v| fe(&v)))
            .collect(),
    );
    out.rec(
        &format!("{p}_is_square"),
        e.iter()
            .map(|x| if bool::from(x.sqrt().is_some()) { "1" } else { "0" }.to_string())
            .collect(),
    );
    out.rec(
        &format!("{p}_sqrt_square"),
        e.iter()
            .map(|x| Option::<F>::from(x.sqrt()).map_or("none".to_string(), |r| fe(&r.square())))
            .collect(),
    );

    // exponent 0xdeadbeefcafebabe12345 as little-endian u64 words
    let exp: u128 = 0xdeadbeefcafebabe12345u128;
    let words = [exp as u64, (exp >> 64) as u64];
    out.rec(
        &format!("{p}_pow"),
        e.iter().map(|x| fe(&x.pow_vartime(words))).collect(),
    );

    let inputs = uniform_inputs();
    out.rec(&format!("{p}_from_uniform_in"), inputs.iter().map(|b| hex(b)).collect());
    out.rec(
        &format!("{p}_from_uniform"),
        inputs.iter().map(|b| fe(&F::from_uniform_bytes(b))).collect(),
    );

    let m = modulus_le::<F>();
    let reprs = [m, add_one_le(m), [0xffu8; 32], sub_one_le(m)];
    out.rec(&format!("{p}_from_repr_in"), reprs.iter().map(|r| hex(r)).collect());
    out.rec(
        &format!("{p}_from_repr_reject"),
        reprs
            .iter()
            .map(|r| {
                Option::<F>::from(F::from_repr(repr_from_bytes::<F>(r)))
                    .map_or("none".to_string(), |v| fe(&v))
            })
            .collect(),
    );
}

pub fn curve_records<C>(p: &str, out: &mut Out)
where
    C: CurveAffine,
    C::Scalar: PrimeField + FromUniformBytes<64>,
    C::Base: PrimeField,
{
    let g = C::generator();
    let aff = |q: C::Curve| pt(&q.to_affine());
    let scalar_elems = elems::<C::Scalar>();

    out.rec(&format!("{p}_generator"), vec![pt(&g)]);
    out.rec(
        &format!("{p}_mul"),
        scalar_elems[..8].iter().map(|k| aff(g * *k)).collect(),
    );

    let small = [1u64, 2, 3, 5, 7, 100];
    let mut add = vec![];
    for i in small {
        for j in small {
            let gi: C::Curve = g * C::Scalar::from(i);
            let gj: C::Curve = g * C::Scalar::from(j);
            add.push(aff(gi + gj));
        }
    }
    out.rec(&format!("{p}_add"), add);

    let mut cur: C::Curve = g.into();
    let mut dbl = vec![];
    for _ in 0..8 {
        cur = cur.double();
        dbl.push(aff(cur));
    }
    out.rec(&format!("{p}_double"), dbl);
    out.rec(&format!("{p}_neg"), vec![aff(-(g * C::Scalar::from(3)))]);

    let enc = |q: C::Curve| -> [u8; 32] { q.to_affine().to_bytes().as_ref().try_into().unwrap() };
    let g1 = enc(g.into());
    let g2 = enc(g * C::Scalar::from(2));
    let g3 = enc(g * C::Scalar::from(3));
    let mut flipped = g1;
    flipped[31] ^= 0x80;
    let non_canonical = modulus_le::<C::Base>();
    let mut x = C::Base::ONE;
    let non_residue = loop {
        let rhs = x.square() * x + C::b();
        if bool::from(rhs.sqrt().is_none()) {
            break x;
        }
        x += C::Base::ONE;
    };
    let mut non_res_bytes = [0u8; 32];
    non_res_bytes.copy_from_slice(non_residue.to_repr().as_ref());
    let mut id_signed = [0u8; 32];
    id_signed[31] = 0x80;
    let inputs = [g1, g2, g3, [0u8; 32], flipped, non_canonical, non_res_bytes, id_signed];
    out.rec(&format!("{p}_decode_in"), inputs.iter().map(|b| hex(b)).collect());
    out.rec(
        &format!("{p}_decode"),
        inputs
            .iter()
            .map(|b| {
                let mut r = <C as GroupEncoding>::Repr::default();
                r.as_mut().copy_from_slice(b);
                Option::<C>::from(C::from_bytes(&r)).map_or("none".to_string(), |q| pt(&q))
            })
            .collect(),
    );

    let five: C = (g * C::Scalar::from(5)).to_affine();
    let c = five.coordinates().unwrap();
    out.rec(&format!("{p}_coordinates"), vec![fe(c.x()), fe(c.y())]);

    let messages = h2c_messages();
    let h = C::CurveExt::hash_to_curve("Halo2-Parameters");
    out.rec(&format!("{p}_h2c"), messages.iter().map(|m| aff(h(m))).collect());
    let t = C::CurveExt::hash_to_curve("z.cash:test");
    out.rec(
        &format!("{p}_h2c_test"),
        [&b"hello"[..], &b""[..]].iter().map(|m| aff(t(m))).collect(),
    );
}

fn h2c_messages() -> Vec<Vec<u8>> {
    let long: Vec<u8> = (0..300).map(|i| (i % 251) as u8).collect();
    vec![
        vec![],
        vec![1],
        vec![2],
        vec![0, 0, 0, 0, 0],
        vec![0, 1, 0, 0, 0],
        vec![0, 2, 0, 0, 0],
        vec![0, 255, 0, 0, 0],
        vec![0, 0, 1, 0, 0],
        vec![0, 0, 0, 0, 1],
        long,
    ]
}

fn params_records<C: CurveAffine>(p: &str, k: u32, out: &mut Out) {
    let params = Params::<C>::new(k);
    let mut buf = vec![];
    params.write(&mut buf).unwrap();
    let n = 1usize << k;
    assert_eq!(buf.len(), 4 + 32 * (2 * n + 2));
    assert_eq!(&buf[..4], &k.to_le_bytes());
    let chunk = |i: usize| hex(&buf[4 + 32 * i..4 + 32 * (i + 1)]);
    let g: Vec<String> = (0..n).map(chunk).collect();
    let g_lagrange: Vec<String> = (n..2 * n).map(chunk).collect();
    let direct: Vec<String> = params.get_g().iter().map(|q| pt(q)).collect();
    assert_eq!(g, direct);
    out.rec(&format!("{p}_params_g"), g);
    out.rec(&format!("{p}_params_g_lagrange"), g_lagrange);
    out.rec(&format!("{p}_params_w"), vec![chunk(2 * n)]);
    out.rec(&format!("{p}_params_u"), vec![chunk(2 * n + 1)]);
}

fn commit_records(out: &mut Out) {
    let params = Params::<EqAffine>::new(4);
    let domain = EvaluationDomain::<Fp>::new(1, 4);
    let values: Vec<Fp> = (0..16u64).map(|i| Fp::from(i * i + 7)).collect();
    let blind = Fp::from(12345) * Fp::from(678910);
    out.rec("commit_poly", values.iter().map(fe).collect());
    out.rec("commit_blind", vec![fe(&blind)]);
    let coeff = domain.coeff_from_vec(values.clone());
    let lagrange = domain.lagrange_from_vec(values);
    out.rec("commit_coeff", vec![pt(&params.commit(&coeff, Blind(blind)).to_affine())]);
    out.rec(
        "commit_lagrange",
        vec![pt(&params.commit_lagrange(&lagrange, Blind(blind)).to_affine())],
    );
}

fn ipa_records(out: &mut Out) {
    let params = Params::<EqAffine>::new(4);
    let domain = EvaluationDomain::<Fp>::new(1, 4);
    let coeffs: Vec<Fp> = (0..16u64).map(|i| Fp::from(i + 1) * Fp::from(i + 1)).collect();
    let px = domain.coeff_from_vec(coeffs);
    let blind = Blind(Fp::from(99));
    let p = params.commit(&px, blind).to_affine();

    let mut transcript = Blake2bWrite::<Vec<u8>, EqAffine, Challenge255<EqAffine>>::init(vec![]);
    transcript.write_point(p).unwrap();
    let x = transcript.squeeze_challenge_scalar::<()>();
    let v = eval_polynomial(&px, *x);
    transcript.write_scalar(v).unwrap();
    create_proof(&params, OsRng, &mut transcript, &px, blind, *x).unwrap();
    let ch_prover = transcript.squeeze_challenge();
    let proof = transcript.finalize();

    let mut t = Blake2bRead::<&[u8], EqAffine, Challenge255<EqAffine>>::init(&proof[..]);
    assert_eq!(p, t.read_point().unwrap());
    let x2 = t.squeeze_challenge_scalar::<()>();
    assert_eq!(*x, *x2);
    assert_eq!(v, t.read_scalar().unwrap());
    let mut msm = params.empty_msm();
    msm.append_term(Fp::ONE, p);
    let guard = verify_proof(&params, msm, &mut t, *x, v).expect("ipa verify");
    let ch_verifier = t.squeeze_challenge();
    assert_eq!(*ch_prover, *ch_verifier);
    assert!(guard.use_challenges().eval());

    out.rec("ipa_p", vec![pt(&p)]);
    out.rec("ipa_x", vec![fe(&*x)]);
    out.rec("ipa_v", vec![fe(&v)]);
    out.rec("ipa_proof", vec![hex(&proof)]);
    out.rec("ipa_next_challenge", vec![hex(&*ch_prover)]);
}

fn blake_records(out: &mut Out) {
    let personals: [&[u8]; 3] = [b"Halo2-Transcript", b"Halo2-Verify-Key", &[0u8; 16]];
    for personal in personals {
        for len in [0usize, 1, 3, 127, 128, 129, 255, 256, 300] {
            let msg: Vec<u8> = (0..len).map(|i| ((i * 7 + 3) % 256) as u8).collect();
            let digest = Blake2bParams::new()
                .hash_length(64)
                .personal(personal)
                .to_state()
                .update(&msg)
                .finalize();
            out.rec(
                "blake2b",
                vec![hex(personal), len.to_string(), hex(digest.as_bytes())],
            );
        }
    }
    let digest = Blake2bParams::new().hash_length(32).to_state().update(b"abc").finalize();
    out.rec("blake2b32_abc", vec![hex(digest.as_bytes())]);

    let g = EqAffine::generator();
    let mut t = Blake2bWrite::<Vec<u8>, EqAffine, Challenge255<EqAffine>>::init(vec![]);
    t.write_point(g).unwrap();
    let c1 = t.squeeze_challenge();
    t.write_scalar(Fp::from(5)).unwrap();
    t.write_scalar(Fp::from(6)).unwrap();
    let c2 = t.squeeze_challenge();
    t.write_point((g * Fp::from(2)).to_affine()).unwrap();
    let proof = t.finalize();
    out.rec("transcript_demo", vec![hex(&*c1), hex(&*c2), hex(&proof)]);
}

pub fn all(out: &mut Out) {
    field_records::<Fp>("fp", out);
    field_records::<Fq>("fq", out);
    curve_records::<EpAffine>("pallas", out);
    curve_records::<EqAffine>("vesta", out);
    params_records::<EqAffine>("vesta", 4, out);
    params_records::<EpAffine>("pallas", 3, out);
    commit_records(out);
    ipa_records(out);
    blake_records(out);
}
