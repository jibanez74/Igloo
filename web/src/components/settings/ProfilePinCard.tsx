import { useId, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { KeyRound } from "lucide-react";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import { updateUserPin } from "@/lib/api";
import { authUserQueryOpts, userPinQueryOpts } from "@/lib/query-opts";
import {
  ADMIN_USERS_KEY,
  AUTH_USER_KEY,
  SETTINGS_CARD_SURFACE_CLASS,
  USER_PIN_KEY,
  USER_PIN_LENGTH,
} from "@/lib/constants";
import {
  showSuccess,
  showActionFailed,
  showValidationError,
} from "@/lib/toast-helpers";
import { lightInputClassName } from "@/lib/input-styles";
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
  const hasPin =
    userData?.error === false && userData.data?.user
      ? userData.data.user.has_pin
      : false;

  const [revealed, setRevealed] = useState(false);
  const [currentPin, setCurrentPin] = useState("");
  const [newPin, setNewPin] = useState("");
  const [errors, setErrors] = useState<PinErrors>({});

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
    showValidationError(message);
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

  const pinMutation = useMutation({
    mutationFn: ({ pin, current }: { pin: string; current?: string }) =>
      updateUserPin(pin, current),
    onSuccess: (res, variables) => {
      const removing = variables.pin === "";
      const action = removing ? "remove PIN" : "update PIN";

      if (res.error) {
        showMutationError(
          variables.current !== undefined,
          action,
          res.message || `Failed to ${action}.`,
        );
        return;
      }

      setErrors({});
      setCurrentPin("");
      setNewPin("");
      setRevealed(false);
      if (res.data?.user) {
        queryClient.setQueryData([AUTH_USER_KEY], {
          error: false,
          data: { user: res.data.user },
        });
      }
      queryClient.invalidateQueries({ queryKey: [USER_PIN_KEY] });
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      showSuccess(removing ? "PIN removed" : "PIN saved");
    },
    onError: (_error, variables) => {
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

    if (!PIN_PATTERN.test(newPin)) {
      showFieldError(
        "newPin",
        `PIN must be exactly ${USER_PIN_LENGTH} digits.`,
        newPinId,
      );
      return;
    }

    if (hasPin && !validateCurrentPin()) {
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
    setRevealed(current => !current);
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
    <Card className={SETTINGS_CARD_SURFACE_CLASS}>
      <SettingsCardHeader
        icon={KeyRound}
        title="Profile PIN"
        description={`An optional ${USER_PIN_LENGTH}-digit PIN that protects your profile on shared TV devices`}
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
          <p className="text-xs text-destructive" role="alert">
            {pinData.message || "Failed to load your PIN."}
          </p>
        )}

        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {hasPin && (
            <div className="space-y-2">
              <Label htmlFor={currentPinId} className="text-muted-foreground">
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
                placeholder="Enter current PIN"
                className={`font-mono ${lightInputClassName}`}
                aria-label="Current PIN"
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
                <p
                  id={currentPinErrorId}
                  className="text-xs text-destructive"
                  role="alert"
                >
                  {errors.currentPin}
                </p>
              )}
            </div>
          )}

          <div className="space-y-2">
            <Label htmlFor={newPinId} className="text-muted-foreground">
              {hasPin ? "New PIN" : "PIN"}
            </Label>
            <Input
              id={newPinId}
              type="text"
              inputMode="numeric"
              value={newPin}
              onChange={e => {
                setNewPin(e.target.value);
                clearError("newPin");
              }}
              placeholder={hasPin ? "Enter new PIN" : "Enter a PIN"}
              className={`font-mono ${lightInputClassName}`}
              aria-label={hasPin ? "New PIN" : "PIN"}
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
            <p
              id={newPinDescriptionId}
              className="text-xs text-muted-foreground"
            >
              Exactly {USER_PIN_LENGTH} digits
            </p>
            {errors.newPin && (
              <p
                id={newPinErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.newPin}
              </p>
            )}
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <Button
              type="submit"
              disabled={pinMutation.isPending}
              variant="accent"
              className="w-full sm:w-auto"
            >
              {pinMutation.isPending
                ? "Saving..."
                : hasPin
                  ? "Update PIN"
                  : "Set PIN"}
            </Button>
            {hasPin && (
              <Button
                type="button"
                variant="outline"
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
