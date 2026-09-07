import { Component, type ErrorInfo, type ReactNode } from "react";

type PortalErrorBoundaryProps = {
  children: ReactNode;
  page: string;
};

type PortalErrorBoundaryState = {
  error?: Error;
};

export class PortalErrorBoundary extends Component<PortalErrorBoundaryProps, PortalErrorBoundaryState> {
  state: PortalErrorBoundaryState = {};

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Portal page failed", { error, info, page: this.props.page });
  }

  componentDidUpdate(previous: PortalErrorBoundaryProps) {
    if (previous.page !== this.props.page && this.state.error) {
      this.setState({ error: undefined });
    }
  }

  render() {
    if (!this.state.error) return this.props.children;

    return (
      <section className="cc-page-error">
        <span>Page recovered</span>
        <strong>{this.props.page} could not load completely.</strong>
        <p>{this.state.error.message || "A page widget failed. Refresh the workspace or open another page."}</p>
      </section>
    );
  }
}
