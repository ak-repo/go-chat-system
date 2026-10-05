import type { SVGProps } from 'react';

const paths = {
  chats: 'M4 4h16v12H9l-5 4V4m4 4h8m-8 4h5',
  people: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m16-13a4 4 0 0 1 0 8m4 5v-2a4 4 0 0 0-3-4M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
  bell: 'M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9m-9 12a3 3 0 0 0 6 0',
  settings: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8m0-5v2m0 14v2M3 12h2m14 0h2M5.6 5.6 7 7m10 10 1.4 1.4M5.6 18.4 7 17m10-10 1.4-1.4',
  search: 'M21 21l-5-5m2-6a7 7 0 1 1-14 0 7 7 0 0 1 14 0',
  plus: 'M12 5v14M5 12h14',
  newChat: 'M4 4h16v12H9l-5 4V4m8 3v6m-3-3h6',
  back: 'M19 12H5m7-7-7 7 7 7',
  more: 'M12 4v.01M12 12v.01M12 20v.01',
  archive: 'M4 8h16v12H4V8M3 3h18v5H3V3m9 8v6m-3-3 3 3 3-3',
  pin: 'M8 3h8l-1 6 4 4v2H5v-2l4-4-1-6m4 12v7',
  muted: 'M4 9h4l5-4v14l-5-4H4V9m13 0 5 6m0-6-5 6',
  send: 'm3 3 19 9-19 9 4-9-4-9m4 9h15',
  check: 'm4 12 5 5L20 6',
  checks: 'm2 12 5 5L18 6m-6 11L23 6',
  clock: 'M12 8v4l3 2m6-2a9 9 0 1 1-18 0 9 9 0 0 1 18 0',
  close: 'm6 6 12 12M6 18 18 6',
  logout: 'M9 4H4v16h5m5-12 4 4-4 4m-6-4h13',
  sun: 'M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1 1m12 12 1 1M5 19l1-1M18 6l1-1m-3 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
  moon: 'M20 15A9 9 0 0 1 9 4a9 9 0 1 0 11 11',
} as const;
export type IconName = keyof typeof paths;
export default function Icon({ name, ...props }: SVGProps<SVGSVGElement> & { name: IconName }) {
  return <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={name === 'more' ? 3.5 : 1.8} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}><path d={paths[name]} /></svg>;
}
