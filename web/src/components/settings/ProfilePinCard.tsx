import { useId, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FieldError, FieldHint } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { KeyRound } from "lucide-react";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import { sessionEndedSince, sessionStamp } from "@/hooks/useEndSession";
import { useFocusHandoff } from "@/hooks/useFocusHandoff";
import { updateUserPin } from "@/lib/api";
import { apiErrorMessage } from "@/lib/is-api-failure";
import {
  authUserFrom,
  authUserQueryOpts,
  setSignedInUser,
  userPinQueryOpts,
} from "@/lib/query-opts";
import {
  MOTION_SETTINGS_SURFACE_CLASS,
  USER_PIN_LENGTH,
} from "@/lib/constants";
import {
  showSuccess,
  showActionFailed,
} from "@/lib/toast-helpers";
import { describedBy } from "@/lib/utils";

type PinErrorField = "currentPin" | "newPin";
type PinErrors = Partial<Record<PinErrorField, string>>;

const PIN_PATTERN = /^\d{4}$/;

/**
 * Account-settings card for the optional 4-digit profile PIN that protects a
 * user's profile on shared TV devices. Supports set, change, remove, and an
 * on-demand reveal of the current PIN (a session-only endpoint the TV's
 * device token can never call).
 */
export default function ProfilePinCard() {
  const queryClient = useQueryClient();
  const { data: userData } = useQuery(authUserQueryOpts());
  const hasPin = authUserFrom(userData)?.has_pin ?? false;

  const [revealed, setRevealed] = useState(false);
  const [currentPin, setCurrentPin] = useState("");
  const [newPin, setNewPin] = useState("");
  const [errors, setErrors] = useState<PinErrors>({});
  // "Set PIN" and "Remove PIN" disable themselves while they save, and
  // Remove is gone once the PIN is; the PIN field takes focus first.
  const formRef = useRef<HTMLFormElement | null>(null);
  const newPinInputRef = useRef<HTMLInputElement | null>(null);
  useFocusHandoff(formRef, newPinInputRef);

  const currentPinId = useId();
  const newPinId = useId();
  const newPinDescriptionId = `${newPinId}-description`;
  const currentPinErrorId = `${currentPinId}-error`;
  const newPinErrorId = `${newPinId}-error`;

  const { data: pinData, isPending: pinPending } = useQuery({
    ...userPinQueryOpts(),
    enabled: hasPin && revealed,
  });
  const revealedPin =
    pinData?.error === false ? pinData.data?.pin : null;

  const showFieldError = (
    field: PinErrorField,
    message: string,
    fieldId: string,
  ) => {
    setErrors(current => ({ ...current, [field]: message }));
    document.getElementById(fieldId)?.focus();
  };

  const clearError = (field: PinErrorField) => {
    setErrors(current => ({ ...current, [field]: undefined }));
  };

  const showMutationError = (
    hasCurrentPin: boolean,
    action: string,
    message: string,
  ) => {
    const field = hasCurrentPin ? "currentPin" : "newPin";
    const fieldId = hasCurrentPin ? currentPinId : newPinId;
    setErrors({ [field]: message });
    showActionFailed(action, message);
    document.getElementById(fieldId)?.focus();
  };

  // Drops the plaintext PIN from the cache without fetching it again, so the
  // next reveal waits for a fresh copy ("…") instead of showing this one.
  // resetQueries would refetch at once: the reveal's observer is still
  // enabled while the click that hides it is handled.
  const forgetPin = () => {
    queryClient
      .getQueryCache()
      .find({ queryKey: userPinQueryOpts().queryKey, exact: true })
      ?.reset();
  };

  const pinMutation = useMutation({
    mutationFn: ({ pin, current }: { pin: string; current?: string }) =>
      updateUserPin(pin, current),
    onMutate: sessionStamp,
    onSuccess: (res, variables, stamp) => {
      if (sessionEndedSince(stamp)) return;
      const removing = variables.pin === "";
      const action = removing ? "remove PIN" : "update PIN";

      if (res.error) {
        showMutationError(
          variables.current !== undefined,
          action,
          apiErrorMessage(res, `Failed to ${action}.`),
        );
        return;
      }

      setErrors({});
      setCurrentPin("");
      setNewPin("");
      setRevealed(false);
      forgetPin();
      setSignedInUser(queryClient, res.data.user);
      showSuccess(removing ? "PIN removed" : "PIN saved");
    },
    onError: (_error, variables, stamp) => {
      if (sessionEndedSince(stamp)) return;
      const removing = variables.pin === "";
      const action = removing ? "remove PIN" : "update PIN";
      showMutationError(
        variables.current !== undefined,
        action,
        "An unexpected error occurred",
      );
    },
  });

  const validateCurrentPin = () => {
    if (!currentPin) {
      showFieldError(
        "currentPin",
        "Current PIN is required.",
        currentPinId,
      );
      return false;
    }
    if (!PIN_PATTERN.test(currentPin)) {
      showFieldError(
        "currentPin",
        `Current PIN must be exactly ${USER_PIN_LENGTH} digits.`,
        currentPinId,
      );
      return false;
    }
    return true;
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    // In the order the fields are in, so the first invalid one takes focus.
    if (hasPin && !validateCurrentPin()) {
      return;
    }

    if (!PIN_PATTERN.test(newPin)) {
      showFieldError(
        "newPin",
        `PIN must be exactly ${USER_PIN_LENGTH} digits.`,
        newPinId,
      );
      return;
    }

    setErrors({});
    pinMutation.mutate({
      pin: newPin,
      current: hasPin ? currentPin : undefined,
    });
  };

  const handleRemove = () => {
    if (!validateCurrentPin()) {
      return;
    }

    setErrors({});
    pinMutation.mutate({ pin: "", current: currentPin });
  };

  const handleToggleReveal = () => {
    if (revealed) forgetPin();
    setRevealed(!revealed);
  };

  let revealedPinDisplay = "••••";
  if (revealed) {
    if (pinPending) {
      revealedPinDisplay = "…";
    } else if (revealedPin) {
      revealedPinDisplay = revealedPin;
    }
  }

  return (
    <Card className={MOTION_SETTINGS_SURFACE_CLASS}>
      <SettingsCardHeader
        icon={KeyRound}
        title="Profile PIN"
        description={`An optional ${USER_PIN_LENGTH}-digit PIN that protects your profile on shared TV devices.`}
      />
      <CardContent className="max-w-2xl space-y-4">
        {hasPin ? (
          <div className="flex items-center gap-3">
            <p className="text-sm text-muted-foreground">
              Current PIN:{" "}
              <span className="font-mono text-base tracking-widest text-foreground">
                {revealedPinDisplay}
              </span>
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleToggleReveal}
              aria-pressed={revealed}
            >
              {revealed ? "Hide PIN" : "Show PIN"}
            </Button>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            You have not set a profile PIN.
          </p>
        )}
        {revealed && pinData?.error && (
          <FieldError>
            {apiErrorMessage(pinData, "Failed to load your PIN.")}
          </FieldError>
        )}

        <form
          ref={formRef}
          onSubmit={handleSubmit}
          noValidate
          className="space-y-4"
        >
          {hasPin && (
            <div className="space-y-2">
              <Label htmlFor={currentPinId} tone="muted">
                Current PIN
              </Label>
              <Input
                id={currentPinId}
                type="text"
                inputMode="numeric"
                value={currentPin}
                onChange={e => {
                  setCurrentPin(e.target.value);
                  clearError("currentPin");
                }}
                variant="glass"
                className="font-mono"
                autoComplete="off"
                maxLength={USER_PIN_LENGTH}
                required
                aria-required="true"
                aria-invalid={!!errors.currentPin || undefined}
                aria-describedby={describedBy(
                  errors.currentPin && currentPinErrorId,
                )}
              />
              {errors.currentPin && (
                <FieldError id={currentPinErrorId}>
                  {errors.currentPin}
                </FieldError>
              )}
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor={newPinId} tone="muted">
              {hasPin ? "New PIN" : "PIN"}
            </Label>
            <FieldHint id={newPinDescriptionId}>
              Exactly {USER_PIN_LENGTH} digits.
            </FieldHint>
            <Input
              ref={newPinInputRef}
              id={newPinId}
              type="text"
              inputMode="numeric"
              value={newPin}
              onChange={e => {
                setNewPin(e.target.value);
                clearError("newPin");
              }}
              variant="glass"
              className="font-mono"
              autoComplete="off"
              maxLength={USER_PIN_LENGTH}
              required
              aria-required="true"
              aria-invalid={!!errors.newPin || undefined}
              aria-describedby={describedBy(
                newPinDescriptionId,
                errors.newPin && newPinErrorId,
              )}
            />
            {errors.newPin && (
              <FieldError id={newPinErrorId}>{errors.newPin}</FieldError>
            )}
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <Button
              type="submit"
              disabled={pinMutation.isPending}
              variant="accent"
              className="w-full sm:w-auto"
            >
              {pinMutation.isPending ? <Spinner aria-hidden="true" /> : null}
              {pinMutation.isPending
                ? "Saving..."
                : hasPin
                  ? "Update PIN"
                  : "Set PIN"}
            </Button>
            {hasPin && (
              <Button
                type="button"
                variant="outline-destructive"
                onClick={handleRemove}
                disabled={pinMutation.isPending}
                className="w-full sm:w-auto"
              >
                Remove PIN
              </Button>
            )}
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
