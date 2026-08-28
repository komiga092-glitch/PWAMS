// HTMX Type Declarations
// https://htmx.org/docs/

declare namespace htmx {
  interface HtmxConfig {
    attributesEnabled?: boolean;
    defaultSettleDelay?: number;
    defaultSwapDelay?: number;
    defaultTransition?: boolean;
    historyEnabled?: boolean;
    historyCacheEnabled?: boolean;
    includeIndicatorStyles?: boolean;
    indicatorClass?: string;
    requestClass?: string;
    settlingClass?: string;
    swappingClass?: string;
    triggerSpecsCache?: unknown;
  }

  interface HtmxExtension {
    onEvent?: (name: string, event: unknown) => boolean | void;
    transformRequest?: (request: unknown) => unknown;
    transformResponse?: (response: unknown) => unknown;
    isInlineSwap?: (swapStyle: string) => boolean;
    handleSwap?: (swapStyle: string, target: HTMLElement, fragment: unknown, settleInfo: unknown) => boolean | void;
    handleAttributes?: (hxAttribute: string, element: HTMLElement) => void;
  }

  function defineExtension(name: string, extension: HtmxExtension): void;
  function removeExtension(name: string): void;
  function getExtension(name: string): HtmxExtension | undefined;

  function trigger(element: HTMLElement, event: string, detail?: unknown): boolean;
  function ajax(config: {
    method?: string;
    url: string;
    target?: HTMLElement;
    swap?: string;
    values?: Record<string, unknown>;
    headers?: Record<string, string>;
  }): void;

  function config(): HtmxConfig;
  function process(element?: HTMLElement): void;
  function on(event: string, callback: (event: Event) => void): void;
}

interface HtmxRequestEvent {
  detail: {
    xhr: XMLHttpRequest;
    target: HTMLElement;
    requestConfig: {
      boosted: boolean;
      useUrlParams: boolean;
      parameters: Record<string, string>;
      unfilteredParameters: Record<string, string>;
      headers: Record<string, string>;
      target: HTMLElement;
      verb: string;
      errors: Array<{ parameter: string; value: string; message: string }>;
      withCredentials: boolean;
      timeout: number;
      path: string;
      triggeringEvent: Event;
      elt: HTMLElement;
    };
    pathInfo: {
      requestPath: string;
      finalRequestPath: string;
      anchor: string;
    };
  };
}

interface HtmxResponseEvent {
  detail: {
    xhr: XMLHttpRequest;
    target: HTMLElement;
    requestConfig: HtmxRequestEvent["detail"]["requestConfig"];
    pathInfo: HtmxRequestEvent["detail"]["pathInfo"];
    shouldSwap: boolean;
    serverResponse: string;
    isError: boolean;
    error?: unknown;
    filter?: (element: HTMLElement) => boolean;
  };
}

declare global {
  interface WindowEventMap {
    "htmx:configRequest": CustomEvent<HtmxRequestEvent["detail"]>;
    "htmx:beforeRequest": CustomEvent<HtmxRequestEvent["detail"]>;
    "htmx:afterRequest": CustomEvent<HtmxRequestEvent["detail"]>;
    "htmx:beforeSwap": CustomEvent<HtmxResponseEvent["detail"]>;
    "htmx:afterSwap": CustomEvent<HtmxResponseEvent["detail"]>;
    "htmx:beforeHistoryUpdate": CustomEvent<unknown>;
    "htmx:afterSettle": CustomEvent<unknown>;
    "htmx:afterSwap": CustomEvent<unknown>;
    "htmx:responseError": CustomEvent<HtmxResponseEvent["detail"]>;
    "htmx:sendError": CustomEvent<HtmxRequestEvent["detail"]>;
    "htmx:oobError": CustomEvent<unknown>;
  }
}

export {};
