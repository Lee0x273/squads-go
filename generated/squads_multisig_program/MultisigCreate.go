// Code generated by https://github.com/gagliardetto/anchor-go. DO NOT EDIT.

package squads_multisig_program

import (
	"errors"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_format "github.com/gagliardetto/solana-go/text/format"
	ag_treeout "github.com/gagliardetto/treeout"
)

// Create a multisig.
type MultisigCreate struct {

	// [0] = [] null
	ag_solanago.AccountMetaSlice `bin:"-"`
}

// NewMultisigCreateInstructionBuilder creates a new `MultisigCreate` instruction builder.
func NewMultisigCreateInstructionBuilder() *MultisigCreate {
	nd := &MultisigCreate{
		AccountMetaSlice: make(ag_solanago.AccountMetaSlice, 1),
	}
	return nd
}

// SetNullAccount sets the "null" account.
func (inst *MultisigCreate) SetNullAccount(null ag_solanago.PublicKey) *MultisigCreate {
	inst.AccountMetaSlice[0] = ag_solanago.Meta(null)
	return inst
}

// GetNullAccount gets the "null" account.
func (inst *MultisigCreate) GetNullAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst MultisigCreate) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: Instruction_MultisigCreate,
	}}
}

// ValidateAndBuild validates the instruction parameters and accounts;
// if there is a validation error, it returns the error.
// Otherwise, it builds and returns the instruction.
func (inst MultisigCreate) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *MultisigCreate) Validate() error {
	// Check whether all (required) accounts are set:
	{
		if inst.AccountMetaSlice[0] == nil {
			return errors.New("accounts.Null is not set")
		}
	}
	return nil
}

func (inst *MultisigCreate) EncodeToTree(parent ag_treeout.Branches) {
	parent.Child(ag_format.Program(ProgramName, ProgramID)).
		//
		ParentFunc(func(programBranch ag_treeout.Branches) {
			programBranch.Child(ag_format.Instruction("MultisigCreate")).
				//
				ParentFunc(func(instructionBranch ag_treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params[len=0]").ParentFunc(func(paramsBranch ag_treeout.Branches) {})

					// Accounts of the instruction:
					instructionBranch.Child("Accounts[len=1]").ParentFunc(func(accountsBranch ag_treeout.Branches) {
						accountsBranch.Child(ag_format.Meta("null", inst.AccountMetaSlice.Get(0)))
					})
				})
		})
}

func (obj MultisigCreate) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	return nil
}
func (obj *MultisigCreate) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	return nil
}

// NewMultisigCreateInstruction declares a new MultisigCreate instruction with the provided parameters and accounts.
func NewMultisigCreateInstruction(
	// Accounts:
	null ag_solanago.PublicKey) *MultisigCreate {
	return NewMultisigCreateInstructionBuilder().
		SetNullAccount(null)
}
