import * as anchor from "@project-serum/anchor";
import { ProgramError, BN } from "@project-serum/anchor";
import {
	SYSVAR_RENT_PUBKEY,
	PublicKey,
	Keypair,
	SystemProgram,
  Transaction,
  TransactionInstruction,
} from '@solana/web3.js';
import {
  Token,
  ASSOCIATED_TOKEN_PROGRAM_ID,
  TOKEN_PROGRAM_ID,
} from '@solana/spl-token';
import { assert } from "chai";

import { randomBytes, createHash } from "crypto";
import * as secp256k1 from "secp256k1";
import { keccak256 } from "ethereum-cryptography/keccak";

// generate a new keypair using `solana-keygen new -o id.json`

describe('ocr2', async () => {

  // Configure the client to use the local cluster.
  const provider = anchor.Provider.env();
  anchor.setProvider(provider);

  const billingAccessController = Keypair.generate();
  const requesterAccessController = Keypair.generate();
  const validator = Keypair.generate();
  const flaggingThreshold = 80000;

  const observationPayment = 1;

  const state = Keypair.generate();
  // const stateSize = 8 + ;
  const transmissions = Keypair.generate();
  const payer = Keypair.generate();
  // const owner = Keypair.generate();
  const owner = provider.wallet;
  const mintAuthority = Keypair.generate();

  const placeholder = Keypair.generate().publicKey;

  const decimals = 18;
  const description = "ETH/BTC";
  
  let token: Token, tokenClient: Token,
    vaultAuthority: PublicKey, vaultNonce: number,
    validatorAuthority: PublicKey, validatorNonce: number,
    tokenVault: PublicKey;
    
  const program = anchor.workspace.Ocr2;
  const accessController = anchor.workspace.AccessController;
  const deviationFlaggingValidator = anchor.workspace.DeviationFlaggingValidator;
  
  const minAnswer = 1;
  const maxAnswer = 1000;


  // Fund the payer
  it('funds the payer', async () => {
    await provider.connection.confirmTransaction(
      await provider.connection.requestAirdrop(payer.publicKey, 10000000000),
      "confirmed"
    );
  });
  
  it('Creates the LINK token', async () => {
    // Create the LINK token
    token = await Token.createMint(
      provider.connection,
      payer,
      mintAuthority.publicKey,
      null,
      decimals,
      TOKEN_PROGRAM_ID,
    );

    tokenClient = new Token(
      provider.connection,
      token.publicKey,
      TOKEN_PROGRAM_ID,
      // @ts-ignore
      program.provider.wallet.payer
    );
  });

  it('Creates the access controllers', async () => {
    await accessController.rpc.initialize({
      accounts: {
        state: billingAccessController.publicKey,
        payer: provider.wallet.publicKey,
        owner: owner.publicKey,
        rent: SYSVAR_RENT_PUBKEY,
        systemProgram: SystemProgram.programId,
      },
      signers: [billingAccessController],
      instructions: [
        await accessController.account.accessController.createInstruction(billingAccessController),
      ],
    });
    await accessController.rpc.initialize({
      accounts: {
        state: requesterAccessController.publicKey,
        payer: provider.wallet.publicKey,
        owner: owner.publicKey,
        rent: SYSVAR_RENT_PUBKEY,
        systemProgram: SystemProgram.programId,
      },
      signers: [requesterAccessController],
      instructions: [
        await accessController.account.accessController.createInstruction(requesterAccessController),
      ],
    });
  });

  it('Creates the validator', async () => {
    [validatorAuthority, validatorNonce] = await PublicKey.findProgramAddress(
      [Buffer.from(anchor.utils.bytes.utf8.encode("validator")), state.publicKey.toBuffer()],
      program.programId
    );

    await deviationFlaggingValidator.rpc.initialize(
      {
      accounts: {
        state: validator.publicKey,
        owner: owner.publicKey,
        raisingAccessController: billingAccessController.publicKey,
        loweringAccessController: billingAccessController.publicKey,
      },
      signers: [validator],
      instructions: [
        await deviationFlaggingValidator.account.validator.createInstruction(validator),
      ],
    });

    [vaultAuthority, vaultNonce] = await PublicKey.findProgramAddress(
      [Buffer.from(anchor.utils.bytes.utf8.encode("vault")), state.publicKey.toBuffer()],
      program.programId
    );
  });

  it('Creates the token vault', async () => {
    // Create an associated token account for LINK, owned by the program instance
    tokenVault = await Token.getAssociatedTokenAddress(
      ASSOCIATED_TOKEN_PROGRAM_ID,
      TOKEN_PROGRAM_ID,
      token.publicKey,
      vaultAuthority,
      true, // allowOwnerOffCurve: seems required since a PDA isn't a valid keypair
    );
  });

  it('Initializes an OCR2 feed', async () => {
    console.log("state", state.publicKey.toBase58());
    console.log("transmissions", transmissions.publicKey.toBase58());
    console.log("payer", provider.wallet.publicKey.toBase58());
    console.log("owner", owner.publicKey.toBase58());
    console.log("tokenMint", token.publicKey.toBase58());
    console.log("tokenVault", tokenVault.toBase58());
    console.log("vaultAuthority", vaultAuthority.toBase58());
    console.log("placeholder", placeholder.toBase58());

    await program.rpc.initialize(vaultNonce, new BN(minAnswer), new BN(maxAnswer), decimals, description, {
      accounts: {
        state: state.publicKey,
        transmissions: transmissions.publicKey,
        payer: provider.wallet.publicKey,
        owner: owner.publicKey,
        tokenMint: token.publicKey,
        tokenVault: tokenVault,
        vaultAuthority: vaultAuthority,
        requesterAccessController: requesterAccessController.publicKey,
        billingAccessController: billingAccessController.publicKey,
        rent: SYSVAR_RENT_PUBKEY,
        systemProgram: SystemProgram.programId,
        tokenProgram: TOKEN_PROGRAM_ID,
        associatedTokenProgram: ASSOCIATED_TOKEN_PROGRAM_ID,
      },
      signers: [state, transmissions],
      instructions: [
        await program.account.state.createInstruction(state),
        await program.account.transmissions.createInstruction(transmissions),
      ],
    });

    let account = await program.account.state.fetch(state.publicKey);
    let config = account.config;
    assert.ok(config.minAnswer.toNumber() == minAnswer);
    assert.ok(config.maxAnswer.toNumber() == maxAnswer);
    assert.ok(config.decimals == 18);

    const f = 2;
    // NOTE: 17 is the most we can fit into one setConfig if we use a different payer
    // if the owner == payer then we can fit 19
    const n = 19; // min: 3 * f + 1;

    console.log(`Generating ${n} oracles...`);
    let oracles = [];
    for (let i = 0; i < n; i++) {
      let secretKey = randomBytes(32);
      let transmitter = Keypair.generate();
      oracles.push({
        signer: {
          secretKey,
          publicKey: secp256k1.publicKeyCreate(secretKey, false).slice(1), // compressed = false, skip first byte (0x04)
        },
        transmitter,
        // Initialize a token account
        payee: await token.getOrCreateAssociatedAccountInfo(transmitter.publicKey),
      });
    }

    const onchain_config = Buffer.from([1, 2, 3]);
    const offchain_config_version = 1;
    const offchain_config = Buffer.from([4, 5, 6]);

    // Fund the owner with LINK tokens
    await token.mintTo(
      tokenVault,
      mintAuthority.publicKey,
      [mintAuthority],
      1000000000
    );

    // TODO: listen for SetConfig event
    
    console.log("beginOffchainConfig");
    await program.rpc.beginOffchainConfig( 
      new BN(offchain_config_version),
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
    });
    console.log("writeOffchainConfig");
    await program.rpc.writeOffchainConfig( 
      offchain_config,
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
    });
    console.log("writeOffchainConfig");
    await program.rpc.writeOffchainConfig( 
      offchain_config,
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
    });
    console.log("commitOffchainConfig");
    await program.rpc.commitOffchainConfig( 
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
    });
    account = await program.account.state.fetch(state.publicKey);
    config = account.config;
    console.log(config);
    assert.ok(config.offchainConfig.len == 6);
    assert.deepEqual(config.offchainConfig.xs.slice(0, config.offchainConfig.len), [4,5,6,4,5,6]);

    let ethereumAddress = (publicKey: Buffer) => {
      return keccak256(publicKey).slice(12)
    };
    
    // 3 byte header + 32+32 addresses + 64 program_id + 64 byte signature + 32 byte block hash + 897 bytes
    // 17 = 897
    // 18 = 949 => serializes to 1249
    // so 300 byte overhead -> 73 bytes unaccounted (64 + 9?)
    // if we ensure owner == feePayer we save some space
    let i = await program.instruction.setConfig(oracles.map((oracle) => ({
      signer: ethereumAddress(Buffer.from(oracle.signer.publicKey)),
      transmitter: oracle.transmitter.publicKey,
    })), f, 
      // onchain_config, 
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
        signers: [],
              });
    console.log(i.data.length);
    console.log(Array.from(i.data));
      

    // Call setConfig
    console.log("setConfig");
    await program.rpc.setConfig(oracles.map((oracle) => ({
      signer: ethereumAddress(Buffer.from(oracle.signer.publicKey)),
      transmitter: oracle.transmitter.publicKey,
    })), f,
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
        signers: [],
    });
    console.log("setPayees")
    await program.rpc.setPayees(
      oracles.map((oracle) => oracle.payee.address),
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
        signers: [],
    });
    console.log("setBilling")
    await program.rpc.setBilling(
      new BN(1),
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
          accessController: billingAccessController.publicKey,
        },
        signers: [],
    });
    console.log("setValidatorConfig");
    await program.rpc.setValidatorConfig(flaggingThreshold,
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
          validator: validator.publicKey,
        },
        signers: [],
    });

    // add the feed to the validator
    console.log("Adding feed to validator access list");
    await accessController.rpc.addAccess({
      accounts: {
        state: billingAccessController.publicKey,
        owner: owner.publicKey,
        address: validatorAuthority,
      },
      signers: [],
    });

    account = await program.account.state.fetch(state.publicKey);
    console.log(account);

		// // log raw state account data
		// let rawAccount = await provider.connection.getAccountInfo(state.publicKey);
		// console.dir([...rawAccount.data], {'maxArrayLength': null})

    // Generate and transmit a report
    let report_context = [];
    report_context.push(...account.config.latestConfigDigest);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0) // 27 byte padding
    report_context.push(0, 0, 0, 1) // epoch 1
    report_context.push(1); //  round 1
    // extra_hash 32 bytes
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);
    report_context.push(0, 0, 0, 0, 0, 0, 0, 0);

    const raw_report = [
      97, 91, 43, 83, // observations_timestamp
      7, // observer_count
      0, 1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // observers
      0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 210, // median
      0, 0, 0, 0, 0, 0, 0, 2, // juels per lamport (2)
    ];

    let hash = createHash('sha256')
      .update(Buffer.from(raw_report))
      .update(Buffer.from(report_context))
      .digest();

    let raw_signatures = [];
    for (let oracle of oracles.slice(0, 3 * f + 1)) { // sign with `f` * 3 + 1 oracles
      // console.log("signer", oracle.signer.publicKey);
      let { signature, recid } = secp256k1.ecdsaSign(hash, oracle.signer.secretKey);
      raw_signatures.push(...signature);
      raw_signatures.push(recid);
    }

    const transmitter = oracles[0].transmitter;


    const tx = new Transaction();
    tx.add(
      new TransactionInstruction({
        programId: anchor.translateAddress(program.programId),
        keys: [
          { pubkey: state.publicKey, isWritable: true, isSigner: false },
          { pubkey: transmitter.publicKey, isWritable: false, isSigner: true },
          { pubkey: transmissions.publicKey, isWritable: true, isSigner: false },
          { pubkey: deviationFlaggingValidator.programId, isWritable: false, isSigner: false },
          { pubkey: validator.publicKey, isWritable: true, isSigner: false },
          { pubkey: validatorAuthority, isWritable: false, isSigner: false },
          { pubkey: billingAccessController.publicKey, isWritable: false, isSigner: false },
      ],
      data: Buffer.concat([
          Buffer.from([validatorNonce]),
          Buffer.from(report_context),
          Buffer.from(raw_report),
          Buffer.from(raw_signatures),
        ]),
      })
    );

    try {
      await provider.send(tx, [transmitter]);
    } catch (err) {
      // Translate IDL error
      const idlErrors = anchor.parseIdlErrors(program.idl);
      let translatedErr = ProgramError.parse(err, idlErrors);
      if (translatedErr === null) {
        throw err;
      }
      throw translatedErr;
    }
    // await program.rpc.transmit(
    //   validatorNonce,
    //   Buffer.concat(
    //     [
    //       Buffer.from(report_context),
    //       Buffer.from(raw_report),
    //       Buffer.from(raw_signatures),
    //     ]
    //   ),
    //   {
    //     accounts: {
    //       state: state.publicKey,
    //       transmitter: transmitter.publicKey,
    //       transmissions: transmissions.publicKey,

    //       validatorProgram: deviationFlaggingValidator.programId,
    //       validator: validator.publicKey,
    //       validatorAuthority: validatorAuthority,
    //       validatorAccessController: billingAccessController.publicKey,
    //     },
    //     signers: [transmitter],
    //   }
    // );

    const recipient = await token.createAccount(placeholder);
    let recipientTokenAccount = await token.getOrCreateAssociatedAccountInfo(recipient);

    console.log("Withdrawing funds");

    await program.rpc.withdrawFunds(
      new BN(1),
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
          accessController: billingAccessController.publicKey,
          tokenVault: tokenVault,
          vaultAuthority: vaultAuthority,
          recipient: recipientTokenAccount.address,
          tokenProgram: TOKEN_PROGRAM_ID,
        },
        signers: [],
      }
    );

    let acc = await tokenClient.getAccountInfo(tokenVault);
    console.log(acc);
    recipientTokenAccount = await tokenClient.getOrCreateAssociatedAccountInfo(recipient);
    console.log(recipientTokenAccount);
    console.log(recipientTokenAccount.amount.toString(10));
    assert.ok(recipientTokenAccount.amount.toNumber() === 1);

    console.log("Calling setConfig again should move payments over to leftover payments");
    await program.rpc.setConfig(oracles.map((oracle) => ({
      signer: ethereumAddress(Buffer.from(oracle.signer.publicKey)),
      transmitter: oracle.transmitter.publicKey,
    })), f,
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
        },
        signers: [],
    });
    account = await program.account.state.fetch(state.publicKey);
    let leftovers = account.leftoverPayments.slice(0, account.leftoverPaymentsLen);
    for (let leftover of leftovers) {
      assert.ok(leftover.amount.toNumber() !== 0);
    }

    console.log("payRemaining");
    let remaining = leftovers.map((leftover) => { return { pubkey: leftover.payee, isWritable: true, isSigner: false }});

    await program.rpc.payRemaining(
      {
        accounts: {
          state: state.publicKey,
          authority: owner.publicKey,
          accessController: billingAccessController.publicKey,
          tokenVault: tokenVault,
          vaultAuthority: vaultAuthority,
          tokenProgram: TOKEN_PROGRAM_ID,
        },
        remainingAccounts: remaining,
        signers: [],
      }
    );

    account = await program.account.state.fetch(state.publicKey);
    assert.ok(account.leftoverPaymentsLen == 0);

		// // log raw transmissions account data
		// let rawTransmissions = await provider.connection.getAccountInfo(transmissions.publicKey);
		// console.log([...rawTransmissions.data])
  });
});
