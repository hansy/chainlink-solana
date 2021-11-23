use anchor_lang::prelude::*;
use anchor_lang::solana_program::{system_program, sysvar};
use static_assertions::const_assert;
use std::mem;

use arrayvec::arrayvec;

declare_id!("ENmeY9iRUUzN5NUvhHTc5vA8nQuYqXcsU7h4JzKMw5aE");

const MAX_ADDRS: usize = 32;

#[zero_copy]
pub struct AccessList {
    xs: [Pubkey; 32], // sadly we can't use const https://github.com/project-serum/anchor/issues/632
    len: u8,
}
arrayvec!(AccessList, Pubkey, u8);
const_assert!(mem::size_of::<AccessList>() == 1 + mem::size_of::<Pubkey>() * MAX_ADDRS);

#[account(zero_copy)] // TODO: force repr(C) here
pub struct AccessController {
    pub owner: Pubkey,
    pub access_list: AccessList,
}

// IDEA: use a PDA with seeds = [account()], bump = ? to check for proof that account exists
// the tradeoff would be that we would have to calculate the PDA and pass it as an account everywhere

#[program]
pub mod access_controller {
    use super::*;
    pub fn initialize(ctx: Context<Initialize>) -> ProgramResult {
        let mut state = ctx.accounts.state.load_init()?;
        state.owner = ctx.accounts.owner.key();
        Ok(())
    }

    #[access_control(owner(&ctx.accounts.state, &ctx.accounts.owner))]
    pub fn add_access(ctx: Context<AddAccess>) -> ProgramResult {
        let mut state = ctx.accounts.state.load_mut()?;
        // if the len reaches array len, we're at capacity
        require!(state.access_list.remaining_capacity() > 0, Full);

        state.access_list.push(ctx.accounts.address.key());
        // keep the access list sorted so we can use binary search
        state.access_list.sort_unstable();
        Ok(())
    }

    #[access_control(owner(&ctx.accounts.state, &ctx.accounts.owner))]
    pub fn remove_access(ctx: Context<RemoveAccess>) -> ProgramResult {
        let mut state = ctx.accounts.state.load_mut()?;
        let address = ctx.accounts.address.key();

        let index = state.access_list.iter().position(|key| key == &address);
        if let Some(index) = index {
            state.access_list.remove(index);
            // we don't need to sort again since the list is still sorted
        }
        Ok(())
    }
}

/// Check if `address` is on the access control list.
pub fn has_access(loader: &AccountLoader<AccessController>, address: &Pubkey) -> Result<bool> {
    let state = loader.load()?;
    Ok(state.access_list.binary_search(address).is_ok())
}

fn owner(state_loader: &AccountLoader<AccessController>, signer: &AccountInfo) -> Result<()> {
    let config = state_loader.load()?;
    require!(signer.key.eq(&config.owner), Unauthorized);
    Ok(())
}

#[error]
pub enum ErrorCode {
    #[msg("Unauthorized")]
    Unauthorized = 0,

    #[msg("Access list is full")]
    Full = 1,
}

#[derive(Accounts)]
pub struct Initialize<'info> {
    #[account(zero)]
    pub state: AccountLoader<'info, AccessController>,
    pub payer: AccountInfo<'info>,
    #[account(signer)]
    pub owner: AccountInfo<'info>,

    #[account(address = sysvar::rent::ID)]
    pub rent: Sysvar<'info, Rent>,
    #[account(address = system_program::ID)]
    pub system_program: AccountInfo<'info>,
}

#[derive(Accounts)]
pub struct AddAccess<'info> {
    #[account(mut, has_one = owner)]
    pub state: AccountLoader<'info, AccessController>,
    pub owner: Signer<'info>,
    pub address: AccountInfo<'info>,
}

#[derive(Accounts)]
pub struct RemoveAccess<'info> {
    #[account(mut, has_one = owner)]
    pub state: AccountLoader<'info, AccessController>,
    pub owner: Signer<'info>,
    pub address: AccountInfo<'info>,
}
