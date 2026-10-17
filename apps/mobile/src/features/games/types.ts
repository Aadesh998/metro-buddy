import type { Room, RoomPlayer } from './api';

// What every game board receives. Boards send moves with `move`; the server checks them.
export type BoardProps<V> = {
  room: Room<V>;
  view: V;
  me?: RoomPlayer;
  spectator: boolean;
  myId?: string;
  move: (m: unknown) => void;
  moving: boolean;
  now: number;
};
