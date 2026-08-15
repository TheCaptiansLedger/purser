import { useState } from 'react'
import { Info } from 'lucide-react'
import { Modal } from './Modal'

export interface InfoField {
  name: string
  example: string
}

export interface InfoPopoverProps {
  title: string
  fields: InfoField[]
  triggerLabel: string
}

// InfoPopover is a generic "field reference" trigger — an info icon that
// opens a Modal listing {{.Name}} -> example pairs. First consumer is the
// AfterDark/Music rename-template fields (SettingsCard), built generic
// over {name, example} so any other settings field that needs the same
// "here's what you can put here" reference reuses it rather than growing
// a bespoke popover per field.
export function InfoPopover({ title, fields, triggerLabel }: InfoPopoverProps) {
  const [open, setOpen] = useState(false)

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} aria-label={triggerLabel} className="text-text-secondary hover:text-text">
        <Info size={14} />
      </button>
      {open && (
        <Modal title={title} onClose={() => setOpen(false)}>
          <table className="w-full text-body text-text">
            <thead>
              <tr className="text-left text-label text-text-secondary">
                <th className="pb-2 pr-4">Field</th>
                <th className="pb-2">Example</th>
              </tr>
            </thead>
            <tbody>
              {fields.map(field => (
                <tr key={field.name} className="border-t border-border">
                  <td className="py-1.5 pr-4 font-mono text-label">{`{{.${field.name}}}`}</td>
                  <td className="py-1.5 text-text-secondary">{field.example}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Modal>
      )}
    </>
  )
}
