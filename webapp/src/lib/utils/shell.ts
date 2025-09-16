import { garmApi } from '$lib/api/client';

export interface ShellMessage {
  type: 'createShell' | 'shellReady' | 'shellData' | 'shellResize' | 'shellExit' | 'clientShellClosed';
  sessionId?: ArrayBuffer;
  data?: Uint8Array;
  rows?: number;
  cols?: number;
  isError?: boolean;
  message?: string;
}

export interface ShellConnection {
  ws: WebSocket;
  sessionId: ArrayBuffer | null;
  onData: (data: Uint8Array) => void;
  onReady: () => void;
  onExit: () => void;
  onError: (error: string) => void;
  sendData: (data: Uint8Array) => void;
  resize: (cols: number, rows: number) => void;
  close: () => void;
}

// Message type constants matching the Go implementation
const MESSAGE_TYPE_CREATE_SHELL = 0x04;
const MESSAGE_TYPE_SHELL_READY = 0x05;
const MESSAGE_TYPE_SHELL_DATA = 0x06;
const MESSAGE_TYPE_SHELL_RESIZE = 0x07;
const MESSAGE_TYPE_SHELL_EXIT = 0x08;
const MESSAGE_TYPE_CLIENT_SHELL_CLOSED = 0x09;


function arrayBufferToUuid(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const hex = Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
  return [
    hex.substr(0, 8),
    hex.substr(8, 4),
    hex.substr(12, 4),
    hex.substr(16, 4),
    hex.substr(20, 12)
  ].join('-');
}

function createMessage(type: number, data: ArrayBuffer): ArrayBuffer {
  const message = new ArrayBuffer(1 + data.byteLength);
  const view = new DataView(message);
  view.setUint8(0, type);
  new Uint8Array(message, 1).set(new Uint8Array(data));
  return message;
}

function parseMessage(data: ArrayBuffer): { type: number; payload: ArrayBuffer } {
  const view = new DataView(data);
  if (data.byteLength < 1) {
    throw new Error('Message too short');
  }
  return {
    type: view.getUint8(0),
    payload: data.slice(1)
  };
}


function createShellDataMessage(sessionId: ArrayBuffer, shellData: Uint8Array): ArrayBuffer {
  const data = new ArrayBuffer(16 + shellData.length);
  
  // Copy session ID (16 bytes)
  new Uint8Array(data, 0, 16).set(new Uint8Array(sessionId));
  
  // Copy shell data
  new Uint8Array(data, 16).set(shellData);
  
  return createMessage(MESSAGE_TYPE_SHELL_DATA, data);
}

function createShellResizeMessage(sessionId: ArrayBuffer, cols: number, rows: number): ArrayBuffer {
  const data = new ArrayBuffer(20);
  const view = new DataView(data);
  
  // Copy session ID (16 bytes)
  new Uint8Array(data, 0, 16).set(new Uint8Array(sessionId));
  
  // Set rows and cols (2 bytes each, big endian)
  view.setUint16(16, rows, false);
  view.setUint16(18, cols, false);
  
  return createMessage(MESSAGE_TYPE_SHELL_RESIZE, data);
}

function createClientShellClosedMessage(sessionId: ArrayBuffer): ArrayBuffer {
  const data = new ArrayBuffer(16);
  
  // Copy session ID (16 bytes)
  new Uint8Array(data).set(new Uint8Array(sessionId));
  
  return createMessage(MESSAGE_TYPE_CLIENT_SHELL_CLOSED, data);
}

export function createShellConnection(
  runnerName: string,
  onData: (data: Uint8Array) => void,
  onReady: () => void,
  onExit: () => void,
  onError: (error: string) => void
): Promise<ShellConnection> {
  return new Promise((resolve, reject) => {
    try {
      // Create WebSocket URL - convert HTTP(S) to WS(S) and rely on cookie authentication
      const baseUrl = window.location.origin.replace(/^http/, 'ws');
      const wsUrl = `${baseUrl}/api/v1/ws/agent/${encodeURIComponent(runnerName)}/shell`;
      
      const ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';
      
      let sessionId: ArrayBuffer | null = null;
      let isConnected = false;

      const connection: ShellConnection = {
        ws,
        sessionId: null,
        onData,
        onReady,
        onExit,
        onError,
        sendData: (data: Uint8Array) => {
          if (sessionId && ws.readyState === WebSocket.OPEN) {
            const message = createShellDataMessage(sessionId, data);
            ws.send(message);
          }
        },
        resize: (cols: number, rows: number) => {
          if (sessionId && ws.readyState === WebSocket.OPEN) {
            const message = createShellResizeMessage(sessionId, cols, rows);
            ws.send(message);
          }
        },
        close: () => {
          if (sessionId && ws.readyState === WebSocket.OPEN) {
            const message = createClientShellClosedMessage(sessionId);
            ws.send(message);
          }
          ws.close();
        }
      };

      ws.onopen = () => {
        isConnected = true;
        // Server automatically handles CreateShell message when connection is established
        // Client just waits for ShellReady message
      };

      ws.onmessage = (event) => {
        try {
          const { type, payload } = parseMessage(event.data);
          
          switch (type) {
            case MESSAGE_TYPE_SHELL_READY:
              if (payload.byteLength >= 17) {
                const view = new DataView(payload);
                const receivedSessionId = payload.slice(0, 16);
                const isError = view.getUint8(16);
                const message = payload.byteLength > 17
                  ? new TextDecoder('utf-8').decode(payload.slice(17))
                  : '';

                if (isError) {
                  onError(message || 'Shell initialization failed');
                } else {
                  // Store the session ID received from the server
                  sessionId = receivedSessionId;
                  connection.sessionId = sessionId;
                  onReady();
                }
              }
              break;
              
            case MESSAGE_TYPE_SHELL_DATA:
              if (payload.byteLength >= 16) {
                const shellData = new Uint8Array(payload.slice(16));
                onData(shellData);
              }
              break;
              
            case MESSAGE_TYPE_SHELL_EXIT:
              onExit();
              break;
              
            default:
              console.warn('Unknown message type:', type);
              break;
          }
        } catch (err) {
          onError(`Failed to parse message: ${err instanceof Error ? err.message : 'Unknown error'}`);
        }
      };

      ws.onerror = (event) => {
        onError('WebSocket error occurred');
      };

      ws.onclose = (event) => {
        if (!isConnected) {
          reject(new Error(`Failed to connect: ${event.reason || 'Connection closed'}`));
        } else {
          onExit();
        }
      };

      resolve(connection);
      
    } catch (err) {
      reject(err);
    }
  });
}