export declare const goGlobal: typeof globalThis;
export declare class Go {
	run(instance: WebAssembly.WebAssemblyInstantiatedSource["instance"]): Promise<void>;
	importObject: WebAssembly.Imports
}
