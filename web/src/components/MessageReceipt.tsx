import type { MessageStatus } from '../api/messages';
import Icon from './Icon';
export default function MessageReceipt({ status = 'sent' }: { status?: MessageStatus }) {
  return <span className={`message-receipt${status === 'read' ? ' is-read' : ''}${status === 'failed' ? ' is-failed' : ''}`} aria-label={status} title={status}><Icon name={status === 'sending' ? 'clock' : status === 'failed' ? 'close' : status === 'sent' ? 'check' : 'checks'} width={17} height={17} /></span>;
}
