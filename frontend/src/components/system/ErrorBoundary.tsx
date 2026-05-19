import { Component, type ErrorInfo, type ReactNode } from 'react';

import { ErrorState } from './ErrorState';

/**
 * Catches render-time exceptions from any descendant and surfaces them
 * as the canonical <ErrorState /> panel instead of giving the user a
 * blank white screen. Wrap any route that pulls data from queries with
 * complex render-side derivations (charts, custom layouts) so a single
 * missing field can't take down the whole tab.
 *
 * onReset is fired when the user clicks "Try again" — the parent is
 * responsible for resetting whatever state caused the failure (e.g.
 * clearing a bad URL param). The boundary itself only resets its own
 * caught-error state.
 */

interface ErrorBoundaryProps {
  children: ReactNode;
  /** Optional override for the panel title. */
  title?: string;
  /** Optional override for the panel description. */
  description?: string;
  onReset?: () => void;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // Surface the error to dev tooling. In production we lean on the
    // visible ErrorState; no telemetry sink wired in this repo yet.
    // eslint-disable-next-line no-console
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  reset = (): void => {
    this.setState({ error: null });
    this.props.onReset?.();
  };

  render(): ReactNode {
    if (this.state.error) {
      return (
        <ErrorState
          title={this.props.title ?? 'Something went wrong rendering this view'}
          description={
            this.props.description ??
            `Try again, or pick a different selection. The error was: ${this.state.error.message}`
          }
          onRetry={this.reset}
        />
      );
    }
    return this.props.children;
  }
}
