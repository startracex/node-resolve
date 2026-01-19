export declare const goGlobal: typeof globalThis;
export declare class Go {
	run(instance: WebAssembly.Instance): Promise<void>;
	importObject: WebAssembly.Imports
}
